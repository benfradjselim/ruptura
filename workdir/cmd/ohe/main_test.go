package main

import (
	"testing"
)

func TestGenerateSecureSecret(t *testing.T) {
	length := 32
	secret := generateSecureSecret(length)
	
	// Hex encoded, so length should be 2 * input length
	if len(secret) != length*2 {
		t.Errorf("expected length %d, got %d", length*2, len(secret))
	}
	
	// Ensure it's not empty
	if secret == "" {
		t.Error("expected non-empty secret")
	}
}
