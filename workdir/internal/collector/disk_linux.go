//go:build linux

package collector

import "golang.org/x/sys/unix"

type diskStatResult struct {
	used, total uint64
}

func getDiskStat(path string) (diskStatResult, error) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return diskStatResult{}, err
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	return diskStatResult{used: used, total: total}, nil
}
