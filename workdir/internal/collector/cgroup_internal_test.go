package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCgroupUint64(t *testing.T) {
	dir := t.TempDir()

	// Valid file with a uint64 value
	valid := filepath.Join(dir, "memory.usage_in_bytes")
	os.WriteFile(valid, []byte("1048576\n"), 0o644)
	if got := readCgroupUint64(valid); got != 1048576 {
		t.Errorf("readCgroupUint64(valid) = %d; want 1048576", got)
	}

	// Non-existent file → 0
	if got := readCgroupUint64(filepath.Join(dir, "missing")); got != 0 {
		t.Errorf("readCgroupUint64(missing) = %d; want 0", got)
	}

	// Empty file → 0
	empty := filepath.Join(dir, "empty")
	os.WriteFile(empty, []byte(""), 0o644)
	if got := readCgroupUint64(empty); got != 0 {
		t.Errorf("readCgroupUint64(empty) = %d; want 0", got)
	}
}
