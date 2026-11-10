package main

import "testing"

func TestConsoleEnabled(t *testing.T) {
	off, on := false, true
	tests := []struct {
		name    string
		enabled *bool
		want    bool
	}{
		{"on by default", nil, true},
		{"explicitly on", &on, true},
		{"disabled", &off, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := consoleEnabled(RupturaInstanceSpec{Console: ConsoleConfig{Enabled: tt.enabled}}); got != tt.want {
				t.Fatalf("consoleEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsoleImage(t *testing.T) {
	tests := []struct {
		name, spec, related, want string
	}{
		{"built-in default", "", "", defaultConsoleImage},
		{"image passed by OLM", "", "mirror.example/ruptura-ui@sha256:abc", "mirror.example/ruptura-ui@sha256:abc"},
		{"instance spec wins", "registry.example/ui:1", "mirror.example/ruptura-ui@sha256:abc", "registry.example/ui:1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(relatedConsoleImageEnv, tt.related)
			if got := consoleImage(RupturaInstanceSpec{Console: ConsoleConfig{Image: tt.spec}}); got != tt.want {
				t.Fatalf("consoleImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConsoleDeployment_StaysOutOfTheEngineService(t *testing.T) {
	dep := consoleDeployment("ruptura-system", "ruptura", "img", true, nil)
	labels := dep.Spec.Template.Metadata.Labels
	if _, ok := labels[instanceLabel]; ok {
		t.Fatalf("console pod labels %v carry %s, so the engine Service would route to them", labels, instanceLabel)
	}
	if dep.Metadata.Name != "ruptura-console" {
		t.Fatalf("name = %q", dep.Metadata.Name)
	}
	c := dep.Spec.Template.Spec.Containers[0]
	if len(c.Env) != 1 || c.Env[0].Value != "http://ruptura:8080" {
		t.Fatalf("env = %+v, want the engine Service as backend", c.Env)
	}
	if sc := dep.Spec.Template.Spec.SecurityContext; sc.RunAsUser != 0 {
		t.Fatalf("OpenShift assigns the UID, got runAsUser %d", sc.RunAsUser)
	}
	svc := consoleService("ruptura-system", "ruptura", nil)
	if svc.Spec.Selector[consoleLabel] != "ruptura" || len(svc.Spec.Selector) != 1 {
		t.Fatalf("service selector = %v", svc.Spec.Selector)
	}
}
