package edge

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type metricsCollector struct {
	prevCPUIdle   uint64
	prevCPUTotal  uint64
	cacheDiskPath string
}

func newMetricsCollector(cacheDiskPath string) *metricsCollector {
	return &metricsCollector{
		cacheDiskPath: cacheDiskPath,
	}
}

type systemMetrics struct {
	CPUUsage  float64
	MemUsage  float64
	DiskUsage float64
	LoadAvg   float64
}

func (mc *metricsCollector) collect() systemMetrics {
	return systemMetrics{
		CPUUsage:  mc.collectCPU(),
		MemUsage:  collectMem(),
		DiskUsage: mc.collectDisk(),
		LoadAvg:   collectLoad(),
	}
}

func (mc *metricsCollector) collectCPU() float64 {
	idle, total, err := readCPUStats()
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

func readCPUStats() (idle, total uint64, err error) {
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
	return 0
}
