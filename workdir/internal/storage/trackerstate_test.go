package storage

import (
	"bytes"
	"testing"
)

func TestTrackerState(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got, err := store.GetTrackerState(); err != nil || got != nil {
		t.Fatalf("empty store: %q, %v, want nil, nil", got, err)
	}
	for _, want := range [][]byte{[]byte("first"), []byte("second")} {
		if err := store.PutTrackerState(want); err != nil {
			t.Fatal(err)
		}
		got, err := store.GetTrackerState()
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("GetTrackerState = %q, %v, want %q", got, err, want)
		}
	}
}
