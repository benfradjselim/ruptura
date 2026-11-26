package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRequireToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := requireToken("s3cret", ok)
	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"right token", "Bearer s3cret", http.StatusOK},
		{"no header", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"token without scheme", "s3cret", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestSecretFromFile(t *testing.T) {
	path := t.TempDir() + "/key"
	if err := os.WriteFile(path, []byte("  abc \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RUPTURA_TEST_FILE", path)
	if got, err := secret("RUPTURA_TEST"); err != nil || got != "abc" {
		t.Fatalf("secret = %q, %v", got, err)
	}
}
