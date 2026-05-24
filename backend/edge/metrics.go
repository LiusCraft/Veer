package edge

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type metricsCollector struct {
	prevCPUIdle   uint64
	prevCPUTotal  uint64
	prevTxBytes   uint64
	prevRxBytes   uint64
	cacheDiskPath string
	firstNetwork  bool
}

func newMetricsCollector(cacheDiskPath string) *metricsCollector {
	return &metricsCollector{
		cacheDiskPath: cacheDiskPath,
		firstNetwork:  true,
	}
}

type systemMetrics struct {
	CPUUsage    float64
	MemUsage    float64
	DiskUsage   float64
	LoadAvg     float64
	TxBytes     int64
	RxBytes     int64
	CacheHits   int64
	CacheMisses int64
}

func (mc *metricsCollector) collect() systemMetrics {
	return systemMetrics{
		CPUUsage:  mc.collectCPU(),
		MemUsage:  collectMem(),
		DiskUsage: mc.collectDisk(),
		LoadAvg:   collectLoad(),
		TxBytes:   mc.collectNetworkTx(),
		RxBytes:   mc.collectNetworkRx(),
	}
}

func (mc *metricsCollector) collectCPU() float64 {
	// Try cgroupv2 first (preferred in modern Docker/Linux)
	if usage := collectCgroupV2CPU(); usage >= 0 {
		return usage
	}
	// Fall back to cgroupv1
	if usage := collectCgroupV1CPU(); usage >= 0 {
		return usage
	}
	// Final fallback: host /proc/stat (not container-aware)
	return mc.collectHostCPU()
}

func (mc *metricsCollector) collectHostCPU() float64 {
	idle, total, err := readHostCPUStats()
	if err != nil {
		return 0
	}

	if mc.prevCPUTotal == 0 {
		mc.prevCPUIdle = idle
		mc.prevCPUTotal = total
		return 0
	}

	deltaIdle := idle - mc.prevCPUIdle
	deltaTotal := total - mc.prevCPUTotal

	mc.prevCPUIdle = idle
	mc.prevCPUTotal = total

	if deltaTotal == 0 {
		return 0
	}

	usage := (1 - float64(deltaIdle)/float64(deltaTotal)) * 100
	return math.Round(usage*10) / 10
}

func readHostCPUStats() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("unexpected /proc/stat format")
		}
		for i := 1; i < len(fields); i++ {
			val, _ := strconv.ParseUint(fields[i], 10, 64)
			total += val
			if i == 4 {
				idle = val
			}
		}
		return idle, total, scanner.Err()
	}
	return 0, 0, fmt.Errorf("no cpu line in /proc/stat")
}

// collectCgroupV2CPU reads container CPU usage from cgroupv2 interfaces.
// Returns -1 if cgroupv2 is not available.
func collectCgroupV2CPU() float64 {
	// Read cpu.stat for actual usage
	usage, err := readUint64File("/sys/fs/cgroup/cpu.stat", "usage_usec")
	if err != nil {
		return -1
	}
	// Read cpu.max for quota (format: "quota period" or "max period")
	quota, period, err := readCPUMax("/sys/fs/cgroup/cpu.max")
	if err != nil || quota <= 0 || period <= 0 {
		return -1
	}

	// Collect twice with a short interval to get a delta
	// First call: just record baseline
	if cgroupV2PrevUsage == 0 {
		cgroupV2PrevUsage = usage
		cgroupV2PrevTime = time.Now()
		return 0
	}

	elapsed := time.Since(cgroupV2PrevTime).Microseconds()
	if elapsed <= 0 {
		return 0
	}

	delta := usage - cgroupV2PrevUsage
	cgroupV2PrevUsage = usage
	cgroupV2PrevTime = time.Now()

	if delta <= 0 {
		return 0
	}

	// quota is in microseconds per period (e.g., 100000 = 100ms)
	// Calculate how many periods elapsed
	periods := float64(elapsed) / float64(period)
	maxCPU := float64(quota) * periods

	pct := (float64(delta) / maxCPU) * 100
	if pct > 100 {
		pct = 100
	}
	return math.Round(pct*10) / 10
}

var (
	cgroupV2PrevUsage uint64
	cgroupV2PrevTime  time.Time
)

// collectCgroupV1CPU reads container CPU usage from cgroupv1 interfaces.
// Returns -1 if cgroupv1 is not available.
func collectCgroupV1CPU() float64 {
	usage, err := readUint64File("/sys/fs/cgroup/cpuacct/cpuacct.usage", "")
	if err != nil {
		return -1
	}

	quota, err := readInt64File("/sys/fs/cgroup/cpu/cpu.cfs_quota_us")
	if err != nil || quota <= 0 {
		return -1
	}
	period, err := readInt64File("/sys/fs/cgroup/cpu/cpu.cfs_period_us")
	if err != nil || period <= 0 {
		return -1
	}

	if cgroupV1PrevUsage == 0 {
		cgroupV1PrevUsage = usage
		cgroupV1PrevTime = time.Now()
		return 0
	}

	elapsed := time.Since(cgroupV1PrevTime).Microseconds()
	if elapsed <= 0 {
		return 0
	}

	delta := usage - cgroupV1PrevUsage // nanoseconds
	cgroupV1PrevUsage = usage
	cgroupV1PrevTime = time.Now()

	if delta <= 0 {
		return 0
	}

	cores := float64(quota) / float64(period)
	maxCPU := cores * float64(elapsed) * 1000 // elapsed in µs, need ns

	pct := (float64(delta) / maxCPU) * 100
	if pct > 100 {
		pct = 100
	}
	return math.Round(pct*10) / 10
}

var (
	cgroupV1PrevUsage uint64
	cgroupV1PrevTime  time.Time
)

// readCPUMax reads a cgroupv2 cpu.max file (format: "quota period" or "max period").
func readCPUMax(path string) (quota, period int64, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 {
		return 0, 0, fmt.Errorf("unexpected cpu.max format: %s", string(data))
	}
	if fields[0] == "max" {
		return 0, 0, fmt.Errorf("cpu.max is unlimited")
	}
	q, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	p, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return q, p, nil
}

// readUint64File reads a uint64 value from a file, optionally matching a key prefix.
func readUint64File(path, key string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if key == "" {
		return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, key) {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.ParseUint(fields[1], 10, 64)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("key %s not found in %s", key, path)
}

// readInt64File reads an int64 value from a file.
func readInt64File(path string) (int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
}

func collectMem() float64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	var total, available uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = parseMemValue(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			available = parseMemValue(line)
		}
	}
	if err := scanner.Err(); err != nil {
		return 0
	}
	if total == 0 {
		return 0
	}
	usage := (1 - float64(available)/float64(total)) * 100
	return math.Round(usage*10) / 10
}

func parseMemValue(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	val, _ := strconv.ParseUint(fields[1], 10, 64)
	return val
}

func (mc *metricsCollector) collectDisk() float64 {
	path := mc.cacheDiskPath
	if path == "" {
		path = "."
	}
	// resolve symlinks to get the real mount point
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(abs, &stat); err != nil {
		return 0
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total == 0 {
		return 0
	}
	usage := (1 - float64(free)/float64(total)) * 100
	return math.Round(usage*10) / 10
}

func (mc *metricsCollector) collectNetworkTx() int64 {
	tx, _, err := readNetStats()
	if err != nil {
		return 0
	}
	if mc.firstNetwork {
		mc.prevTxBytes = tx
		return 0
	}
	delta := int64(tx - mc.prevTxBytes)
	mc.prevTxBytes = tx
	if delta < 0 {
		return 0
	}
	return delta
}

func (mc *metricsCollector) collectNetworkRx() int64 {
	_, rx, err := readNetStats()
	if err != nil {
		return 0
	}
	if mc.firstNetwork {
		mc.prevRxBytes = rx
		mc.firstNetwork = false
		return 0
	}
	delta := int64(rx - mc.prevRxBytes)
	mc.prevRxBytes = rx
	if delta < 0 {
		return 0
	}
	return delta
}

// readNetStats reads total tx/rx bytes from /proc/net/dev, summing all interfaces.
func readNetStats() (tx, rx uint64, err error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Skip header lines
	scanner.Scan()
	scanner.Scan()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		// fields[0] is interface name (e.g., "eth0:"), fields[1] is rx_bytes, fields[9] is tx_bytes
		iface := fields[0]
		// Skip loopback
		if strings.HasPrefix(iface, "lo:") {
			continue
		}
		rxVal, _ := strconv.ParseUint(fields[1], 10, 64)
		txVal, _ := strconv.ParseUint(fields[9], 10, 64)
		rx += rxVal
		tx += txVal
	}
	return tx, rx, scanner.Err()
}

type hardwareSpec struct {
	CPUCores   int
	MemoryMB   int64
	DiskSizeMB int64
}

func detectHardware(cacheDiskPath string) hardwareSpec {
	cores := runtime.NumCPU()

	var memTotal int64
	if total, err := readMemTotalBytes(); err == nil {
		memTotal = int64(total / 1024) // kB → MB
	} else {
		memTotal = 8192
	}

	var diskTotal int64
	if path := cacheDiskPath; path != "" {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		var stat syscall.Statfs_t
		if err := syscall.Statfs(abs, &stat); err == nil {
			diskTotal = int64(stat.Blocks * uint64(stat.Bsize) / (1024 * 1024))
		}
	}
	if diskTotal <= 0 {
		diskTotal = 102400 // 100 GB default
	}

	return hardwareSpec{CPUCores: cores, MemoryMB: memTotal, DiskSizeMB: diskTotal}
}

func readMemTotalBytes() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			return parseMemValue(line), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, fmt.Errorf("MemTotal not found")
}

func collectLoad() float64 {
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 0 {
			val, err := strconv.ParseFloat(fields[0], 64)
			if err == nil {
				return math.Round(val*10) / 10
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return 0
	}
	return 0
}
