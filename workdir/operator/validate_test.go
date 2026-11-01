package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestValidateSpec(t *testing.T) {
	channel := func(name string) AlertChannel {
		return AlertChannel{Name: name, Alertmanager: &AlertmanagerChannel{URL: "http://am:9093"}}
	}
	cases := []struct {
		name    string
		spec    RupturaInstanceSpec
		wantErr string
	}{
		{"empty spec uses defaults", RupturaInstanceSpec{}, ""},
		{"full valid spec", RupturaInstanceSpec{
			Deployment: DeploymentConfig{Mode: "Deployment"},
			Level:      LevelConfig{Scope: "namespace", Selector: LabelSelector{MatchLabels: map[string]string{"team": "shop"}}},
			Input:      InputConfig{Profile: "standard"},
			Analysis:   AnalysisConfig{Sensitivity: "balanced", Horizon: "30m"},
			Alerting:   AlertingConfig{Channels: []AlertChannel{channel("ops")}},
			Logging:    LoggingConfig{Level: "info"},
		}, ""},
		{"unknown scope", RupturaInstanceSpec{Level: LevelConfig{Scope: "galaxy"}}, "level.scope"},
		{"scope of a later version", RupturaInstanceSpec{Level: LevelConfig{Scope: "node"}}, "not available"},
		{"deep profile of a later version", RupturaInstanceSpec{Input: InputConfig{Profile: "deep"}}, "not available"},
		{"unknown profile", RupturaInstanceSpec{Input: InputConfig{Profile: "max"}}, "input.profile"},
		{"unknown deployment mode", RupturaInstanceSpec{Deployment: DeploymentConfig{Mode: "Job"}}, "deployment.mode"},
		{"unknown sensitivity", RupturaInstanceSpec{Analysis: AnalysisConfig{Sensitivity: "high"}}, "analysis.sensitivity"},
		{"bad horizon", RupturaInstanceSpec{Analysis: AnalysisConfig{Horizon: "soon"}}, "analysis.horizon"},
		{"zero horizon", RupturaInstanceSpec{Analysis: AnalysisConfig{Horizon: "0s"}}, "analysis.horizon"},
		{"unknown log level", RupturaInstanceSpec{Logging: LoggingConfig{Level: "trace"}}, "logging.level"},
		{"channel without name", RupturaInstanceSpec{Alerting: AlertingConfig{Channels: []AlertChannel{{Alertmanager: &AlertmanagerChannel{URL: "u"}}}}}, "alerting.channels"},
		{"duplicate channel", RupturaInstanceSpec{Alerting: AlertingConfig{Channels: []AlertChannel{channel("a"), channel("a")}}}, "duplicate"},
		{"channel without target", RupturaInstanceSpec{Alerting: AlertingConfig{Channels: []AlertChannel{{Name: "a"}}}}, "alertmanager"},
	}
	for _, c := range cases {
		err := validateSpec(c.spec)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("%s: error = %v, want %q", c.name, err, c.wantErr)
		}
	}
}

func TestValidateSpec_ReportsEveryProblem(t *testing.T) {
	err := validateSpec(RupturaInstanceSpec{Level: LevelConfig{Scope: "x"}, Logging: LoggingConfig{Level: "y"}})
	if err == nil || !strings.Contains(err.Error(), "level.scope") || !strings.Contains(err.Error(), "logging.level") {
		t.Errorf("error = %v, want both problems", err)
	}
}

func TestReconcile_InvalidSpecDeploysNothing(t *testing.T) {
	c, rec := recordingClient(t, false)
	inst := RupturaInstance{
		Metadata: ObjectMeta{Name: "ri", Namespace: "ruptura-system", Finalizers: []string{finalizer}},
		Spec:     RupturaInstanceSpec{Level: LevelConfig{Scope: "galaxy"}},
	}
	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatalf("an invalid spec must not be retried as an error: %v", err)
	}
	if _, ok := rec.get(http.MethodPatch, testDepPath); ok {
		t.Error("a deployment was applied for an invalid spec")
	}
	status, ok := rec.get(http.MethodPatch, "/apis/ruptura.io/v1alpha1/namespaces/ruptura-system/rupturainstances/ri/status")
	if !ok {
		t.Fatal("status was not written")
	}
	body := status["status"].(map[string]interface{})
	if body["phase"] != "Failed" || !strings.Contains(body["message"].(string), "level.scope") {
		t.Errorf("status = %v", body)
	}
}

func TestReconcile_ConfigReachesTheEngine(t *testing.T) {
	c, rec := recordingClient(t, false)
	inst := RupturaInstance{
		Metadata: ObjectMeta{Name: "ri", Namespace: "ruptura-system", Finalizers: []string{finalizer}},
		Spec: RupturaInstanceSpec{
			Level:    LevelConfig{Scope: "namespace"},
			Input:    InputConfig{Profile: "minimal"},
			Analysis: AnalysisConfig{Sensitivity: "conservative", Horizon: "45m"},
			Logging:  LoggingConfig{Level: "debug"},
		},
	}
	if err := reconcile(context.Background(), c, inst, false); err != nil {
		t.Fatal(err)
	}
	env := envOf(t, rec)
	want := map[string]string{
		"RUPTURA_LEVEL_SCOPE":   "namespace",
		"RUPTURA_INPUT_PROFILE": "minimal",
		"RUPTURA_SENSITIVITY":   "conservative",
		"RUPTURA_HORIZON":       "45m",
		"LOG_LEVEL":             "debug",
	}
	for name, value := range want {
		if env[name] == nil || env[name]["value"] != value {
			t.Errorf("%s = %v, want %s", name, env[name], value)
		}
	}
}

func TestReconcile_ReaderBinding(t *testing.T) {
	for _, isOCP := range []bool{true, false} {
		c, rec := recordingClient(t, false)
		if err := reconcile(context.Background(), c, makeInstance("ri", testNS), isOCP); err != nil {
			t.Fatal(err)
		}
		crb, ok := rec.get(http.MethodPatch, readerBindingPath(testNS, "ri"))
		if !ok {
			t.Fatalf("isOCP=%v: reader binding was not applied", isOCP)
		}
		if crb["roleRef"].(map[string]interface{})["name"] != readerRole {
			t.Errorf("isOCP=%v: binding = %v", isOCP, crb)
		}
	}
}

func TestCleanup_DeletesReaderBinding(t *testing.T) {
	c, rec := recordingClient(t, false)
	inst := makeInstance("ri", testNS)
	inst.Metadata.Finalizers = []string{finalizer}
	if err := cleanup(c, inst, false); err != nil {
		t.Fatal(err)
	}
	if _, ok := rec.get(http.MethodDelete, readerBindingPath(testNS, "ri")); !ok {
		t.Error("reader binding was not deleted")
	}
}
