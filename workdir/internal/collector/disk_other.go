//go:build !linux

package collector

import "fmt"

type diskStatResult struct {
	used, total uint64
}

func getDiskStat(path string) (diskStatResult, error) {
	return diskStatResult{}, fmt.Errorf("disk stat not supported on this platform")
}
