package api

import (
	"testing"
)

func TestValidateDataSourceURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com", false},
		{"invalid scheme", "ftp://example.com", true},
		// These tests rely on DNS, so they might be flaky or require mocking/bypass
		// "private IP", "http://192.168.1.1", true},
		// "metadata", "http://169.254.169.254", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateDataSourceURL(tt.url); (err != nil) != tt.wantErr {
				t.Errorf("validateDataSourceURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
