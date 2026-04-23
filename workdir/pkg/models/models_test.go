package models

import (
	"testing"
)

func TestLoginRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     LoginRequest
		wantErr bool
	}{
		{"valid", LoginRequest{"user", "pass"}, false},
		{"empty username", LoginRequest{"", "pass"}, true},
		{"empty password", LoginRequest{"user", ""}, true},
		{"all empty", LoginRequest{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPIKeyCreateRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     APIKeyCreateRequest
		wantErr bool
	}{
		{"valid", APIKeyCreateRequest{"key", "admin", ""}, false},
		{"empty name", APIKeyCreateRequest{"", "admin", ""}, true},
		{"empty role", APIKeyCreateRequest{"key", "", ""}, true},
		{"all empty", APIKeyCreateRequest{"", "", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.req.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
