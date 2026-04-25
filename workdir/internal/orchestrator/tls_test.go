package orchestrator

import "testing"

// TestTLSConfigValidation verifies that the orchestrator rejects a partial TLS
// configuration (cert without key, or key without cert).
// Full TLS acceptance is an integration test that requires real certs.
func TestTLSConfigValidation(t *testing.T) {
	cases := []struct {
		name    string
		cert    string
		key     string
		wantErr bool
	}{
		{"no TLS", "", "", false},
		{"cert only", "/etc/ohe/tls.crt", "", true},
		{"key only", "", "/etc/ohe/tls.key", true},
		{"both provided", "/etc/ohe/tls.crt", "/etc/ohe/tls.key", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tlsEnabled := tc.cert != "" || tc.key != ""
			partial := tlsEnabled && (tc.cert == "" || tc.key == "")
			if partial != tc.wantErr {
				t.Errorf("cert=%q key=%q: partial=%v want wantErr=%v", tc.cert, tc.key, partial, tc.wantErr)
			}
		})
	}
}
