package api

import (
	"os"
	"testing"

	"github.com/benfradjselim/ruptura/internal/storage"
)

func newTestStore(t *testing.T) (*storage.Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ruptura-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	s, err := storage.Open(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("open store: %v", err)
	}
	return s, func() {
		s.Close()
		os.RemoveAll(dir)
	}
}
