package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
)

// ContainerCollector reads cgroup-based metrics for running containers.
// It detects Docker containers via the Docker socket if available,
// otherwise falls back to scanning /sys/fs/cgroup for cgroup entries.
type ContainerCollector struct {
	host       string
	dockerSock string
	client     *http.Client
}

// NewContainerCollector creates a container metrics collector
func NewContainerCollector(host string) *ContainerCollector {
	sock := "/var/run/docker.sock"
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", sock)
			},
		},
	}
	return &ContainerCollector{
		host:       host,
		dockerSock: sock,
		client:     client,
	}
}

// dockerAvailable returns true if the Docker socket is accessible
func (c *ContainerCollector) dockerAvailable() bool {
	_, err := os.Stat(c.dockerSock)
	return err == nil
}

// Collect returns per-container metrics where available
func (c *ContainerCollector) Collect() ([]models.Metric, error) {
	if c.dockerAvailable() {
		return c.collectViaDocker()
	}
	return c.collectViaCgroup()
}

// collectViaDocker uses the Docker stats API
func (c *ContainerCollector) collectViaDocker() ([]models.Metric, error) {
	// List containers
	resp, err := c.client.Get("http://docker/containers/json")
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var containers []struct {
		ID    string `json:"Id"`
		Names []string
	}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, fmt.Errorf("decode containers: %w", err)
	}

	now := time.Now()
	var metrics []models.Metric

	for _, ct := range containers {
		name := ct.ID[:12]
		if len(ct.Names) > 0 {
			name = strings.TrimPrefix(ct.Names[0], "/")
		}

		statsResp, err := c.client.Get(fmt.Sprintf("http://docker/containers/%s/stats?stream=false", ct.ID))
		if err != nil {
			continue
		}
		defer func() { _ = statsResp.Body.Close() }()

		var stats dockerStats
		if err := json.NewDecoder(statsResp.Body).Decode(&stats); err != nil {
			continue
		}

		labels := map[string]string{"container": name, "id": ct.ID[:12]}

		// CPU delta
		cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(stats.CPUStats.SystemCPUUsage - stats.PreCPUStats.SystemCPUUsage)
		numCPU := float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
		if numCPU == 0 {
			numCPU = 1
		}
		cpuPercent := 0.0
		if systemDelta > 0 {
			cpuPercent = (cpuDelta / systemDelta) * numCPU * 100.0
		}

		memPercent := 0.0
		if stats.MemoryStats.Limit > 0 {
			memPercent = float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100
		}

		metrics = append(metrics,
			models.Metric{Name: "container_cpu_percent", Value: cpuPercent, Timestamp: now, Host: c.host, Labels: labels},
			models.Metric{Name: "container_mem_percent", Value: memPercent, Timestamp: now, Host: c.host, Labels: labels},
			models.Metric{Name: "container_mem_used_mb",
				Value:     float64(stats.MemoryStats.Usage) / 1048576.0,
				Timestamp: now, Host: c.host, Labels: labels},
		)

		// Network I/O
		for iface, net := range stats.Networks {
			ifLabels := map[string]string{"container": name, "iface": iface}
			metrics = append(metrics,
				models.Metric{Name: "container_net_rx_bytes", Value: float64(net.RxBytes), Timestamp: now, Host: c.host, Labels: ifLabels},
				models.Metric{Name: "container_net_tx_bytes", Value: float64(net.TxBytes), Timestamp: now, Host: c.host, Labels: ifLabels},
			)
		}
	}

	return metrics, nil
}

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64 `json:"usage"`
		Limit uint64 `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

// collectViaCgroup scans /sys/fs/cgroup for container memory/cpu stats
func (c *ContainerCollector) collectViaCgroup() ([]models.Metric, error) {
	cgroupBase := "/sys/fs/cgroup"
	if _, err := os.Stat(cgroupBase); err != nil {
		return nil, nil // cgroup not available
	}

	now := time.Now()
	var metrics []models.Metric

	// Walk docker scope dirs
	_ = filepath.WalkDir(filepath.Join(cgroupBase, "memory", "docker"), func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		containerID := filepath.Base(path)
		if len(containerID) < 12 {
			return nil
		}
		labels := map[string]string{"id": containerID[:12]}

		memUsage := readCgroupUint64(filepath.Join(path, "memory.usage_in_bytes"))
		memLimit := readCgroupUint64(filepath.Join(path, "memory.limit_in_bytes"))
		if memUsage > 0 {
			metrics = append(metrics, models.Metric{
				Name: "container_mem_used_mb", Value: float64(memUsage) / 1048576.0,
				Timestamp: now, Host: c.host, Labels: labels,
			})
			if memLimit > 0 && memLimit < 1<<60 {
				metrics = append(metrics, models.Metric{
					Name: "container_mem_percent", Value: float64(memUsage) / float64(memLimit) * 100,
					Timestamp: now, Host: c.host, Labels: labels,
				})
			}
		}
		return nil
	})

	return metrics, nil
}

func readCgroupUint64(path string) uint64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		var v uint64
		_, _ = fmt.Sscan(sc.Text(), &v)
		return v
	}
	return 0
}
