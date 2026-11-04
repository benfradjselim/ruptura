package main

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"sort"
	"strings"
)

const (
	prometheusMountPath = "/etc/ruptura/prometheus"
	otlpMountPath       = "/etc/ruptura/otlp"
	// inputsHashAnnotation changes when a referenced Secret or ConfigMap
	// changes, so the engine restarts with the rotated values.
	inputsHashAnnotation = "ruptura.io/inputs-hash"
)

// KeySelector names one key of a Secret or ConfigMap in the instance namespace.
type KeySelector struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ValueSource reads a value from exactly one Secret or ConfigMap key.
type ValueSource struct {
	SecretKeyRef    *KeySelector `json:"secretKeyRef,omitempty"`
	ConfigMapKeyRef *KeySelector `json:"configMapKeyRef,omitempty"`
}

// ClientTLS configures how the engine verifies and authenticates to a server.
type ClientTLS struct {
	CA         *ValueSource `json:"ca,omitempty"`
	Cert       *ValueSource `json:"cert,omitempty"`
	Key        *ValueSource `json:"key,omitempty"`
	ServerName string       `json:"serverName,omitempty"`
}

// BasicAuth reads a username and a password.
type BasicAuth struct {
	Username *ValueSource `json:"username"`
	Password *ValueSource `json:"password"`
}

// Authorization selects one way to authenticate to Prometheus.
type Authorization struct {
	BearerToken         *ValueSource `json:"bearerToken,omitempty"`
	BasicAuth           *BasicAuth   `json:"basicAuth,omitempty"`
	ServiceAccountToken bool         `json:"serviceAccountToken,omitempty"`
}

// ServerTLS configures the OTLP receiver certificate and, with ClientCA,
// requires client certificates.
type ServerTLS struct {
	Cert     *ValueSource `json:"cert"`
	Key      *ValueSource `json:"key"`
	ClientCA *ValueSource `json:"clientCA,omitempty"`
}

// OTLPInput configures the OTLP receiver.
type OTLPInput struct {
	TLS *ServerTLS `json:"tls,omitempty"`
}

// ProjectedVolumeSource gathers keys of several Secrets and ConfigMaps.
type ProjectedVolumeSource struct {
	Sources []VolumeProjection `json:"sources"`
}

// VolumeProjection is one Secret or ConfigMap of a projected volume.
type VolumeProjection struct {
	Secret    *ProjectionSource `json:"secret,omitempty"`
	ConfigMap *ProjectionSource `json:"configMap,omitempty"`
}

// ProjectionSource selects keys of one object.
type ProjectionSource struct {
	Name  string      `json:"name"`
	Items []KeyToPath `json:"items"`
}

// valueRule states where a value may live and what it must contain.
type valueRule struct {
	field      string
	source     *ValueSource
	secretOnly bool
	file       string
}

func checkValueSource(rule valueRule) []string {
	src := rule.source
	if src == nil {
		return nil
	}
	if (src.SecretKeyRef == nil) == (src.ConfigMapKeyRef == nil) {
		return []string{rule.field + ": set exactly one of secretKeyRef and configMapKeyRef"}
	}
	if rule.secretOnly && src.SecretKeyRef == nil {
		return []string{rule.field + ": must come from a Secret"}
	}
	ref := src.SecretKeyRef
	if ref == nil {
		ref = src.ConfigMapKeyRef
	}
	if ref.Name == "" || ref.Key == "" {
		return []string{rule.field + ": name and key are required"}
	}
	return nil
}

// prometheusRules lists the referenced values of input.prometheus.
func prometheusRules(p *PrometheusSource) []valueRule {
	var rules []valueRule
	if p.TLS != nil {
		rules = append(rules,
			valueRule{"input.prometheus.tls.ca", p.TLS.CA, false, "ca.crt"},
			valueRule{"input.prometheus.tls.cert", p.TLS.Cert, false, "tls.crt"},
			valueRule{"input.prometheus.tls.key", p.TLS.Key, true, "tls.key"})
	}
	if a := p.Authorization; a != nil {
		rules = append(rules, valueRule{"input.prometheus.authorization.bearerToken", a.BearerToken, true, "token"})
		if a.BasicAuth != nil {
			rules = append(rules,
				valueRule{"input.prometheus.authorization.basicAuth.username", a.BasicAuth.Username, false, "username"},
				valueRule{"input.prometheus.authorization.basicAuth.password", a.BasicAuth.Password, true, "password"})
		}
	}
	return rules
}

// otlpRules lists the referenced values of input.otlp.
func otlpRules(o *OTLPInput) []valueRule {
	if o == nil || o.TLS == nil {
		return nil
	}
	return []valueRule{
		{"input.otlp.tls.cert", o.TLS.Cert, false, "tls.crt"},
		{"input.otlp.tls.key", o.TLS.Key, true, "tls.key"},
		{"input.otlp.tls.clientCA", o.TLS.ClientCA, false, "client-ca.crt"},
	}
}

// checkInputs reports shape problems of the input block, before any object is read.
func checkInputs(input InputConfig) []string {
	var problems []string
	if p := input.Prometheus; p != nil {
		for _, rule := range prometheusRules(p) {
			problems = append(problems, checkValueSource(rule)...)
		}
		if p.TLS != nil {
			if (p.TLS.Cert == nil) != (p.TLS.Key == nil) {
				problems = append(problems, "input.prometheus.tls: cert and key go together")
			}
			if strings.HasPrefix(p.URL, "http://") {
				problems = append(problems, "input.prometheus.tls: the url must use https")
			}
		}
		if a := p.Authorization; a != nil {
			methods := 0
			for _, set := range []bool{a.BearerToken != nil, a.BasicAuth != nil, a.ServiceAccountToken} {
				if set {
					methods++
				}
			}
			if methods != 1 {
				problems = append(problems, "input.prometheus.authorization: set exactly one of bearerToken, basicAuth and serviceAccountToken")
			}
			if a.BasicAuth != nil && (a.BasicAuth.Username == nil || a.BasicAuth.Password == nil) {
				problems = append(problems, "input.prometheus.authorization.basicAuth: username and password are required")
			}
		}
	}
	if o := input.OTLP; o != nil && o.TLS != nil {
		if o.TLS.Cert == nil || o.TLS.Key == nil {
			problems = append(problems, "input.otlp.tls: cert and key are required")
		}
		for _, rule := range otlpRules(o) {
			problems = append(problems, checkValueSource(rule)...)
		}
	}
	return problems
}

// resolvedInputs is what the Deployment needs to read the declared inputs.
type resolvedInputs struct {
	env     []EnvVar
	volumes []Volume
	mounts  []VolumeMount
	hash    string
}

type objectData struct {
	Data       map[string]string `json:"data"`
	BinaryData map[string]string `json:"binaryData"`
}

// readValue fetches one referenced value from the API server.
func readValue(c *k8sClient, ns string, src *ValueSource) ([]byte, error) {
	if ref := src.SecretKeyRef; ref != nil {
		var secret objectData
		if err := c.get(fmt.Sprintf("/api/v1/namespaces/%s/secrets/%s", ns, ref.Name), &secret); err != nil {
			return nil, fmt.Errorf("Secret %s: %w", ref.Name, err)
		}
		encoded, ok := secret.Data[ref.Key]
		if !ok {
			return nil, fmt.Errorf("Secret %s has no key %s", ref.Name, ref.Key)
		}
		return base64.StdEncoding.DecodeString(encoded)
	}
	ref := src.ConfigMapKeyRef
	var configMap objectData
	if err := c.get(fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", ns, ref.Name), &configMap); err != nil {
		return nil, fmt.Errorf("ConfigMap %s: %w", ref.Name, err)
	}
	if value, ok := configMap.Data[ref.Key]; ok {
		return []byte(value), nil
	}
	if encoded, ok := configMap.BinaryData[ref.Key]; ok {
		return base64.StdEncoding.DecodeString(encoded)
	}
	return nil, fmt.Errorf("ConfigMap %s has no key %s", ref.Name, ref.Key)
}

func hasCertificate(data []byte) bool {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return false
		}
		if block.Type == "CERTIFICATE" {
			if _, err := x509.ParseCertificate(block.Bytes); err == nil {
				return true
			}
		}
	}
}

// resolveGroup reads and checks the values mounted under one path.
func resolveGroup(c *k8sClient, ns, volume, mountPath string, rules []valueRule, out *resolvedInputs, digest map[string][]byte) ([]string, map[string]string) {
	var problems []string
	files := map[string]string{}
	values := map[string][]byte{}
	var projection []VolumeProjection
	for _, rule := range rules {
		if rule.source == nil {
			continue
		}
		data, err := readValue(c, ns, rule.source)
		if err != nil {
			problems = append(problems, rule.field+": "+err.Error())
			continue
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			problems = append(problems, rule.field+": the value is empty")
			continue
		}
		values[rule.file] = data
		files[rule.file] = mountPath + "/" + rule.file
		digest[volume+"/"+rule.file] = data
		item := []KeyToPath{{Key: "", Path: rule.file}}
		if ref := rule.source.SecretKeyRef; ref != nil {
			item[0].Key = ref.Key
			projection = append(projection, VolumeProjection{Secret: &ProjectionSource{Name: ref.Name, Items: item}})
		} else {
			item[0].Key = rule.source.ConfigMapKeyRef.Key
			projection = append(projection, VolumeProjection{ConfigMap: &ProjectionSource{Name: rule.source.ConfigMapKeyRef.Name, Items: item}})
		}
	}
	for _, file := range []string{"ca.crt", "client-ca.crt"} {
		if data, ok := values[file]; ok && !hasCertificate(data) {
			problems = append(problems, fmt.Sprintf("%s: no valid PEM certificate in %s", volume, file))
		}
	}
	if cert, ok := values["tls.crt"]; ok {
		if key, ok := values["tls.key"]; ok {
			if _, err := tls.X509KeyPair(cert, key); err != nil {
				problems = append(problems, fmt.Sprintf("%s: certificate and key do not form a valid pair: %v", volume, err))
			}
		}
	}
	if len(projection) > 0 && len(problems) == 0 {
		out.volumes = append(out.volumes, Volume{Name: volume, Projected: &ProjectedVolumeSource{Sources: projection}})
		out.mounts = append(out.mounts, VolumeMount{Name: volume, MountPath: mountPath, ReadOnly: true})
	}
	return problems, files
}

// resolveInputs reads every referenced value, checks it, and returns the
// volumes and environment of the engine. Values are never logged.
func resolveInputs(c *k8sClient, ns string, input InputConfig) (resolvedInputs, error) {
	var out resolvedInputs
	var problems []string
	digest := map[string][]byte{}
	if p := input.Prometheus; p != nil {
		found, files := resolveGroup(c, ns, "prometheus-credentials", prometheusMountPath, prometheusRules(p), &out, digest)
		problems = append(problems, found...)
		auth := "none"
		if a := p.Authorization; a != nil {
			switch {
			case a.ServiceAccountToken:
				auth = "serviceAccount"
			case a.BearerToken != nil:
				auth = "bearer"
			case a.BasicAuth != nil:
				auth = "basic"
			}
		}
		out.env = append(out.env, EnvVar{Name: "RUPTURA_PROMETHEUS_AUTH", Value: auth})
		for file, name := range map[string]string{"ca.crt": "RUPTURA_PROMETHEUS_CA_FILE", "tls.crt": "RUPTURA_PROMETHEUS_CERT_FILE",
			"tls.key": "RUPTURA_PROMETHEUS_KEY_FILE", "token": "RUPTURA_PROMETHEUS_TOKEN_FILE",
			"username": "RUPTURA_PROMETHEUS_USERNAME_FILE", "password": "RUPTURA_PROMETHEUS_PASSWORD_FILE"} {
			if path, ok := files[file]; ok {
				out.env = append(out.env, EnvVar{Name: name, Value: path})
			}
		}
		if p.TLS != nil && p.TLS.ServerName != "" {
			out.env = append(out.env, EnvVar{Name: "RUPTURA_PROMETHEUS_SERVER_NAME", Value: p.TLS.ServerName})
		}
	}
	if rules := otlpRules(input.OTLP); rules != nil {
		found, files := resolveGroup(c, ns, "otlp-tls", otlpMountPath, rules, &out, digest)
		problems = append(problems, found...)
		for file, name := range map[string]string{"tls.crt": "RUPTURA_OTLP_TLS_CERT_FILE", "tls.key": "RUPTURA_OTLP_TLS_KEY_FILE",
			"client-ca.crt": "RUPTURA_OTLP_TLS_CLIENT_CA_FILE"} {
			if path, ok := files[file]; ok {
				out.env = append(out.env, EnvVar{Name: name, Value: path})
			}
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		return resolvedInputs{}, fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	sort.Slice(out.env, func(i, j int) bool { return out.env[i].Name < out.env[j].Name })
	if len(digest) > 0 {
		keys := make([]string, 0, len(digest))
		for key := range digest {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		h := sha256.New()
		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte{0})
			h.Write(digest[key])
			h.Write([]byte{0})
		}
		out.hash = hex.EncodeToString(h.Sum(nil))[:16]
	}
	return out, nil
}
