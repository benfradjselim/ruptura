package logger

import (
	"context"
	"strings"
	"testing"
)

func TestWithContextKeys(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithOrgID(ctx, "org-456")
	ctx = WithUsername(ctx, "alice")

	if got := RequestID(ctx); got != "req-123" {
		t.Errorf("request_id: got %q", got)
	}
	if got := OrgID(ctx); got != "org-456" {
		t.Errorf("org_id: got %q", got)
	}
	if got := Username(ctx); got != "alice" {
		t.Errorf("username: got %q", got)
	}
}

func TestMissingContextKeys(t *testing.T) {
	ctx := context.Background()
	if got := RequestID(ctx); got != "" {
		t.Errorf("expected empty request_id, got %q", got)
	}
}

func TestQuoteIfNeeded(t *testing.T) {
	cases := [][2]string{
		{"hello", "hello"},
		{"hello world", `"hello world"`},
		{"with\nnewline", `"with\nnewline"`},
	}
	for _, c := range cases {
		got := quoteIfNeeded(c[0])
		if got != c[1] {
			t.Errorf("quoteIfNeeded(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestLoggerEmit(t *testing.T) {
	lg := New("test")
	// Should not panic; output goes to stderr which we can't capture easily
	lg.Info("test message", "key", "value")
	lg.Warn("warn message")
	lg.Error("error message", "err", "oops")
}

func TestLoggerCtx(t *testing.T) {
	lg := New("test")
	ctx := WithRequestID(WithOrgID(context.Background(), "org1"), "rid1")
	// Must not panic
	lg.InfoCtx(ctx, "ctx message", "k", "v")
	lg.WarnCtx(ctx, "ctx warn")
	lg.ErrorCtx(ctx, "ctx error", "err", "bad")
}

func TestLogLevelDefault(t *testing.T) {
	lv := logLevel()
	if lv != "info" && lv != "debug" {
		t.Errorf("unexpected log level: %q", lv)
	}
}

func TestNewRequestID(t *testing.T) {
	// newRequestID is in middleware package; test quoteIfNeeded indirectly
	s := quoteIfNeeded("nospace")
	if strings.Contains(s, `"`) {
		t.Errorf("should not be quoted: %q", s)
	}
}
