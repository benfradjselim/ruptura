package collector

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
)

// SystemCollector reads metrics from /proc on Linux, or uses runtime on other platforms.
type SystemCollector struct {
	mu          sync.Mutex // protects all mutable fields below
	host        string
	prevNetStat netStat
	prevCPUStat cpuStat
	prevTime    time.Time
}

type cpuStat struct {
	user, nice, system, idle, iowait, irq, softirq, steal uint64
}

type netStat struct {
	rxBytes, txBytes uint64
}

// NewSystemCollector creates a collector for the given host
func NewSystemCollector(host string) *SystemCollector {
	sc := &SystemCollector{host: host}
	sc.prevCPUStat, _ = readCPUStat()
	sc.prevNetStat, _ = readNetStat()
	sc.prevTime = time.Now()
	return sc
}

// Collect gathers all system metrics and returns them as a slice
func (sc *SystemCollector) Collect() ([]models.Metric, error) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(sc.prevTime).Seconds()
	if elapsed < 0.001 {
		elapsed = 1
	}

	var metrics []models.Metric

	// CPU
	curCPU, err := readCPUStat()
	if err == nil {
		cpuPercent := calcCPUPercent(sc.prevCPUStat, curCPU)
		metrics = append(metrics, models.Metric{
			Name:      "cpu_percent",
			Value:     cpuPercent,
			Timestamp: now,
			Host:      sc.host,
		})
		sc.prevCPUStat = curCPU
	}

	// Memory
	memTotal, memAvail, err := readMemInfo()
	if err == nil && memTotal > 0 {
		memUsed := memTotal - memAvail
		memPercent := float64(memUsed) / float64(memTotal) * 100.0
		metrics = append(metrics,
			models.Metric{Name: "memory_percent", Value: memPercent, Timestamp: now, Host: sc.host},
			models.Metric{Name: "memory_used_mb", Value: float64(memUsed) / 1024.0, Timestamp: now, Host: sc.host},
			models.Metric{Name: "memory_total_mb", Value: float64(memTotal) / 1024.0, Timestamp: now, Host: sc.host},
		)
	}

	// Disk
	diskUsed, diskTotal, err := readDiskUsage("/")
	if err == nil && diskTotal > 0 {
		diskPercent := float64(diskUsed) / float64(diskTotal) * 100.0
		metrics = append(metrics,
			models.Metric{Name: "disk_percent", Value: diskPercent, Timestamp: now, Host: sc.host},
			models.Metric{Name: "disk_used_gb", Value: float64(diskUsed) / 1073741824.0, Timestamp: now, Host: sc.host},
			models.Metric{Name: "disk_total_gb", Value: float64(diskTotal) / 1073741824.0, Timestamp: now, Host: sc.host},
		)
	}

	// Network
	curNet, err := readNetStat()
	if err == nil && elapsed > 0 {
		// Use signed arithmetic to handle counter wrap-around
		rxDelta := int64(curNet.rxBytes) - int64(sc.prevNetStat.rxBytes)
		txDelta := int64(curNet.txBytes) - int64(sc.prevNetStat.txBytes)
		if rxDelta < 0 {
			rxDelta = 0
		}
		if txDelta < 0 {
			txDelta = 0
		}
		rxBps := float64(rxDelta) / elapsed
		txBps := float64(txDelta) / elapsed
		metrics = append(metrics,
			models.Metric{Name: "net_rx_bps", Value: rxBps, Timestamp: now, Host: sc.host},
			models.Metric{Name: "net_tx_bps", Value: txBps, Timestamp: now, Host: sc.host},
		)
		sc.prevNetStat = curNet
	}

	// Load Average
	load1, load5, load15, err := readLoadAvg()
	if err == nil {
		metrics = append(metrics,
			models.Metric{Name: "load_avg_1", Value: load1, Timestamp: now, Host: sc.host},
			models.Metric{Name: "load_avg_5", Value: load5, Timestamp: now, Host: sc.host},
			models.Metric{Name: "load_avg_15", Value: load15, Timestamp: now, Host: sc.host},
		)
	}

	// Uptime
	uptime, err := readUptime()
	if err == nil {
		metrics = append(metrics, models.Metric{Name: "uptime_seconds", Value: uptime, Timestamp: now, Host: sc.host})
	}

	// Process count
	procs := countProcesses()
	metrics = append(metrics, models.Metric{Name: "processes", Value: float64(procs), Timestamp: now, Host: sc.host})

	// Go runtime goroutines
	metrics = append(metrics, models.Metric{
		Name:      "goroutines",
		Value:     float64(runtime.NumGoroutine()),
		Timestamp: now,
		Host:      sc.host,
	})

	sc.prevTime = now
	return metrics, nil
}

// CollectSystemMetrics returns a typed SystemMetrics snapshot
func (sc *SystemCollector) CollectSystemMetrics() (*models.SystemMetrics, error) {
	raw, err := sc.Collect()
	if err != nil {
		return nil, err
	}
	m := &models.SystemMetrics{Host: sc.host, Timestamp: time.Now()}
	for _, metric := range raw {
		switch metric.Name {
		case "cpu_percent":
			m.CPUPercent = metric.Value
		case "memory_percent":
			m.MemoryPercent = metric.Value
		case "memory_used_mb":
			m.MemoryUsedMB = metric.Value
		case "memory_total_mb":
			m.MemoryTotalMB = metric.Value
		case "disk_percent":
			m.DiskPercent = metric.Value
		case "disk_used_gb":
			m.DiskUsedGB = metric.Value
		case "disk_total_gb":
			m.DiskTotalGB = metric.Value
		case "net_rx_bps":
			m.NetRxBps = metric.Value
		case "net_tx_bps":
			m.NetTxBps = metric.Value
		case "load_avg_1":
			m.LoadAvg1 = metric.Value
		case "load_avg_5":
			m.LoadAvg5 = metric.Value
		case "load_avg_15":
			m.LoadAvg15 = metric.Value
		case "processes":
			m.Processes = int(metric.Value)
		case "uptime_seconds":
			m.Uptime = metric.Value
		}
	}
	return m, nil
}

// --- Linux /proc parsers ---

func readCPUStat() (cpuStat, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuStat{}, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			break
		}
		parse := func(s string) uint64 {
			v, _ := strconv.ParseUint(s, 10, 64)
			return v
		}
		return cpuStat{
			user:    parse(fields[1]),
			nice:    parse(fields[2]),
			system:  parse(fields[3]),
			idle:    parse(fields[4]),
			iowait:  parse(fields[5]),
			irq:     parse(fields[6]),
			softirq: parse(fields[7]),
			steal: func() uint64 {
				if len(fields) > 8 {
					v, _ := strconv.ParseUint(fields[8], 10, 64)
					return v
				}
				return 0
			}(),
		}, nil
	}
	return cpuStat{}, fmt.Errorf("cpu line not found")
}

func calcCPUPercent(prev, curr cpuStat) float64 {
	prevIdle := prev.idle + prev.iowait
	currIdle := curr.idle + curr.iowait
	prevTotal := prev.user + prev.nice + prev.system + prevIdle + prev.irq + prev.softirq + prev.steal
	currTotal := curr.user + curr.nice + curr.system + currIdle + curr.irq + curr.softirq + curr.steal

	totalDelta := float64(currTotal - prevTotal)
	idleDelta := float64(currIdle - prevIdle)
	if totalDelta == 0 {
		return 0
	}
	return (totalDelta - idleDelta) / totalDelta * 100.0
}

func readMemInfo() (total, available uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = val
		case "MemAvailable:":
			available = val
		}
	}
	return total, available, nil
}

func readDiskUsage(path string) (used, total uint64, err error) {
	// Use statfs via syscall
	stat, err := getDiskStat(path)
	if err != nil {
		return 0, 0, err
	}
	return stat.used, stat.total, nil
}

func readNetStat() (netStat, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return netStat{}, err
	}
	defer func() { _ = f.Close() }()

	var rxTotal, txTotal uint64
	scanner := bufio.NewScanner(f)
	// Skip header lines
	scanner.Scan()
	scanner.Scan()
	for scanner.Scan() {
		line := scanner.Text()
		// Skip loopback
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "lo:") {
			continue
		}
		colonIdx := strings.Index(trimmed, ":")
		if colonIdx < 0 {
			continue
		}
		fields := strings.Fields(trimmed[colonIdx+1:])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		rxTotal += rx
		txTotal += tx
	}
	return netStat{rxBytes: rxTotal, txBytes: txTotal}, nil
}

func readLoadAvg() (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected format")
	}
	load1, _ = strconv.ParseFloat(fields[0], 64)
	load5, _ = strconv.ParseFloat(fields[1], 64)
	load15, _ = strconv.ParseFloat(fields[2], 64)
	return load1, load5, load15, nil
}

func readUptime() (float64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("unexpected format")
	}
	return strconv.ParseFloat(fields[0], 64)
}

func countProcesses() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				count++
			}
		}
	}
	return count
}
