package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

var (
	availableScopes   = set("pod", "workload", "namespace")
	plannedScopes     = set("container", "service", "node", "cluster", "application", "customResource")
	availableProfiles = set("minimal", "standard")
	plannedProfiles   = set("full", "deep")
	deploymentModes   = set("Deployment")
	sensitivities     = set("conservative", "balanced", "aggressive")
	logLevels         = set("debug", "info", "warn", "error")
)

func set(values ...string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

// checkChoice reports a value that is unknown or planned for a later version.
func checkChoice(field, value string, available, planned map[string]bool) []string {
	switch {
	case value == "" || available[value]:
		return nil
	case planned[value]:
		return []string{fmt.Sprintf("%s: %q is not available in this version", field, value)}
	}
	return []string{fmt.Sprintf("%s: unknown value %q", field, value)}
}

func checkChannels(channels []AlertChannel) []string {
	var problems []string
	seen := make(map[string]bool, len(channels))
	for i, channel := range channels {
		if channel.Name == "" {
			problems = append(problems, fmt.Sprintf("alerting.channels[%d]: name is required", i))
		}
		if seen[channel.Name] {
			problems = append(problems, fmt.Sprintf("alerting.channels[%d]: duplicate name %q", i, channel.Name))
		}
		seen[channel.Name] = true
		if channel.Alertmanager == nil || channel.Alertmanager.URL == "" {
			problems = append(problems, fmt.Sprintf("alerting.channels[%d]: alertmanager.url is required", i))
		}
	}
	return problems
}

// validateSpec returns every problem of a spec, or nil when it can be deployed.
func validateSpec(spec RupturaInstanceSpec) error {
	var problems []string
	problems = append(problems, checkChoice("deployment.mode", spec.Deployment.Mode, deploymentModes, nil)...)
	problems = append(problems, checkChoice("level.scope", spec.Level.Scope, availableScopes, plannedScopes)...)
	problems = append(problems, checkChoice("input.profile", spec.Input.Profile, availableProfiles, plannedProfiles)...)
	problems = append(problems, checkChoice("analysis.sensitivity", spec.Analysis.Sensitivity, sensitivities, nil)...)
	problems = append(problems, checkChoice("logging.level", spec.Logging.Level, logLevels, nil)...)
	if horizon := spec.Analysis.Horizon; horizon != "" {
		if parsed, err := time.ParseDuration(horizon); err != nil || parsed <= 0 {
			problems = append(problems, fmt.Sprintf("analysis.horizon: invalid duration %q", horizon))
		}
	}
	if prometheus := spec.Input.Prometheus; prometheus != nil {
		if parsed, err := url.Parse(prometheus.URL); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			problems = append(problems, fmt.Sprintf("input.prometheus.url: %q is not an http or https URL", prometheus.URL))
		}
	}
	problems = append(problems, checkChannels(spec.Alerting.Channels)...)
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "; "))
}
