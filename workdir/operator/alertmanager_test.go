package main

import (
	"context"
	"net/http"
	"testing"
)

func alertingInstance(channels ...AlertChannel) RupturaInstance {
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	inst.Spec.Alerting = AlertingConfig{Channels: channels}
	return inst
}

var (
	plainChannel = AlertChannel{Name: "team", Alertmanager: &AlertmanagerChannel{URL: "http://am:9093"}}
	tokenChannel = AlertChannel{Name: "cluster", Alertmanager: &AlertmanagerChannel{
		URL: "https://alertmanager-main.openshift-monitoring.svc:9094", ServiceAccountToken: true}}
)

func TestReconcile_AlertmanagerTargetsSplitByToken(t *testing.T) {
	c, rec := recordingClient(t, false)
	if err := reconcile(context.Background(), c, alertingInstance(plainChannel, tokenChannel), true); err != nil {
		t.Fatal(err)
	}
	env := envOf(t, rec)
	want := map[string]string{
		"RUPTURA_ALERTMANAGER_URLS":       "http://am:9093",
		"RUPTURA_ALERTMANAGER_TOKEN_URLS": "https://alertmanager-main.openshift-monitoring.svc:9094",
		"RUPTURA_ALERTMANAGER_CA_FILE":    "/etc/ruptura/service-ca/service-ca.crt",
	}
	for name, value := range want {
		if env[name] == nil || env[name]["value"] != value {
			t.Errorf("%s = %v, want %s", name, env[name], value)
		}
	}
}

func TestReconcile_AlertmanagerAccessOnOCP(t *testing.T) {
	cases := []struct {
		name        string
		channels    []AlertChannel
		wantBinding bool
	}{
		{"token channel binds the edit role", []AlertChannel{tokenChannel}, true},
		{"plain channel removes the binding", []AlertChannel{plainChannel}, false},
	}
	for _, tc := range cases {
		c, rec := recordingClient(t, false)
		if err := reconcile(context.Background(), c, alertingInstance(tc.channels...), true); err != nil {
			t.Fatal(err)
		}
		cm, ok := rec.get(http.MethodPatch, serviceCAConfigMapPath(testNS, "ri"))
		if !ok {
			t.Fatalf("%s: service CA ConfigMap not applied", tc.name)
		}
		annotations := cm["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
		if annotations[serviceCAInjectAnnotation] != "true" {
			t.Errorf("%s: annotations = %v", tc.name, annotations)
		}
		binding, applied := rec.get(http.MethodPatch, alertmanagerBindingPath(testNS, "ri"))
		_, deleted := rec.get(http.MethodDelete, alertmanagerBindingPath(testNS, "ri"))
		if applied != tc.wantBinding || deleted == tc.wantBinding {
			t.Fatalf("%s: applied = %v, deleted = %v", tc.name, applied, deleted)
		}
		if tc.wantBinding {
			ref := binding["roleRef"].(map[string]interface{})
			if ref["kind"] != "Role" || ref["name"] != alertmanagerEditRole {
				t.Errorf("%s: roleRef = %v", tc.name, ref)
			}
		}
	}
}

func TestReconcile_NoServiceCAOffOCP(t *testing.T) {
	c, rec := recordingClient(t, false)
	if err := reconcile(context.Background(), c, alertingInstance(tokenChannel), false); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.get(http.MethodPatch, serviceCAConfigMapPath(testNS, "ri")); ok {
		t.Error("service CA ConfigMap must not be applied off OpenShift")
	}
	if env := envOf(t, rec); env["RUPTURA_ALERTMANAGER_CA_FILE"] != nil {
		t.Errorf("CA file env set off OpenShift: %v", env["RUPTURA_ALERTMANAGER_CA_FILE"])
	}
}

func TestCleanup_DeletesAlertmanagerAccessOnOCP(t *testing.T) {
	c, rec := recordingClient(t, false)
	if err := cleanup(c, alertingInstance(tokenChannel), true); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{alertmanagerBindingPath(testNS, "ri"), serviceCAConfigMapPath(testNS, "ri")} {
		if _, ok := rec.get(http.MethodDelete, path); !ok {
			t.Errorf("%s was not deleted", path)
		}
	}
}
