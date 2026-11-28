// Package agent holds the node agent's readers, the report it sends to the
// engine over OTLP, and the engine's store of the latest agent readings.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// NodePIDs reads the tasks running on the node and the kernel's pid_max from
// a host proc root. The kernel counts every thread against pid_max, and the
// fourth field of loadavg is that count.
func NodePIDs(procRoot string) (used, limit float64, err error) {
	raw, err := os.ReadFile(filepath.Join(procRoot, "loadavg"))
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 4 {
		return 0, 0, fmt.Errorf("agent: loadavg: %q", raw)
	}
	_, total, found := strings.Cut(fields[3], "/")
	if !found {
		return 0, 0, fmt.Errorf("agent: loadavg tasks: %q", fields[3])
	}
	if used, err = strconv.ParseFloat(total, 64); err != nil {
		return 0, 0, err
	}
	if limit, err = readNumber(filepath.Join(procRoot, "sys/kernel/pid_max")); err != nil {
		return 0, 0, err
	}
	return used, limit, nil
}

// PodPIDs is the PID use of one pod's cgroup.
type PodPIDs struct {
	UID   string
	Used  float64
	Limit float64
}

// podSlice matches a pod cgroup of the systemd driver, such as
// kubepods-burstable-pod<uid>.slice, where the uid has underscores.
var podSlice = regexp.MustCompile(`^kubepods(?:-[a-z]+)?-pod([0-9a-f_]+)\.slice$`)

// ScanPodPIDs reads pids.current and pids.max of every pod cgroup under a
// cgroup v2 root. A pod with no PID limit ("max") is left out: it has
// nothing to run out of but the node's own limit.
func ScanPodPIDs(cgroupRoot string) ([]PodPIDs, error) {
	var out []PodPIDs
	root := filepath.Join(cgroupRoot, "kubepods.slice")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		m := podSlice.FindStringSubmatch(d.Name())
		if m == nil {
			return nil
		}
		used, errUsed := readNumber(filepath.Join(path, "pids.current"))
		limit, errLimit := readNumber(filepath.Join(path, "pids.max"))
		if errUsed == nil && errLimit == nil {
			out = append(out, PodPIDs{UID: strings.ReplaceAll(m[1], "_", "-"), Used: used, Limit: limit})
		}
		return filepath.SkipDir
	})
	return out, err
}

func readNumber(path string) (float64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
}
