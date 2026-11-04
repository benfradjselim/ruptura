package main

import (
	"context"
	"strings"
	"testing"
)

func TestPrometheusURL(t *testing.T) {
	declared := RupturaInstanceSpec{Input: InputConfig{Prometheus: &PrometheusSource{URL: "http://prometheus.monitoring.svc:9090"}}}
	cases := []struct {
		name  string
		spec  RupturaInstanceSpec
		isOCP bool
		want  string
	}{
		{"declared on Kubernetes", declared, false, "http://prometheus.monitoring.svc:9090"},
		{"declared on OpenShift wins", declared, true, "http://prometheus.monitoring.svc:9090"},
		{"default on OpenShift", RupturaInstanceSpec{}, true, thanosURL},
		{"none on Kubernetes", RupturaInstanceSpec{}, false, ""},
	}
	for _, tc := range cases {
		if got := prometheusURL(tc.spec, tc.isOCP); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestValidate_PrometheusURL(t *testing.T) {
	cases := []struct {
		url   string
		valid bool
	}{
		{"http://prometheus.monitoring.svc:9090", true},
		{"https://thanos.example.com", true},
		{"", false},
		{"prometheus:9090", false},
		{"ftp://prometheus", false},
		{"http://", false},
	}
	for _, tc := range cases {
		err := validateSpec(RupturaInstanceSpec{Input: InputConfig{Prometheus: &PrometheusSource{URL: tc.url}}})
		if (err == nil) != tc.valid {
			t.Errorf("%q: err = %v, want valid %v", tc.url, err, tc.valid)
		}
		if err != nil && !strings.Contains(err.Error(), "input.prometheus.url") {
			t.Errorf("%q: error does not name the field: %v", tc.url, err)
		}
	}
}

func TestReconcile_PrometheusEnvOnKubernetes(t *testing.T) {
	for _, tc := range []struct {
		name string
		url  string
		want interface{}
	}{
		{"declared", "http://prometheus.monitoring.svc:9090", "http://prometheus.monitoring.svc:9090"},
		{"absent", "", nil},
	} {
		c, rec := recordingClient(t, false)
		inst := makeInstance("ri", testNS)
		inst.Metadata.Finalizers = []string{finalizer}
		if tc.url != "" {
			inst.Spec.Input.Prometheus = &PrometheusSource{URL: tc.url}
		}
		if err := reconcile(context.Background(), c, inst, false); err != nil {
			t.Fatal(err)
		}
		env := envOf(t, rec)
		var got interface{}
		if env["RUPTURA_THANOS_URL"] != nil {
			got = env["RUPTURA_THANOS_URL"]["value"]
		}
		if got != tc.want {
			t.Errorf("%s: RUPTURA_THANOS_URL = %v, want %v", tc.name, got, tc.want)
		}
	}
}
