package main

import (
	"os"

	"github.com/benfradjselim/ruptura/internal/sources"
)

// prometheusConfig reads how to reach the query endpoint. With no setting the
// engine keeps its in-cluster behaviour: service account token and cluster CAs.
func prometheusConfig(url string) sources.PrometheusConfig {
	return sources.PrometheusConfig{
		URL:          url,
		CAFile:       os.Getenv("RUPTURA_PROMETHEUS_CA_FILE"),
		CertFile:     os.Getenv("RUPTURA_PROMETHEUS_CERT_FILE"),
		KeyFile:      os.Getenv("RUPTURA_PROMETHEUS_KEY_FILE"),
		ServerName:   os.Getenv("RUPTURA_PROMETHEUS_SERVER_NAME"),
		Auth:         os.Getenv("RUPTURA_PROMETHEUS_AUTH"),
		TokenFile:    os.Getenv("RUPTURA_PROMETHEUS_TOKEN_FILE"),
		UsernameFile: os.Getenv("RUPTURA_PROMETHEUS_USERNAME_FILE"),
		PasswordFile: os.Getenv("RUPTURA_PROMETHEUS_PASSWORD_FILE"),
	}
}
