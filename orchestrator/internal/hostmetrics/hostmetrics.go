package hostmetrics

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	procStatPath    = "/proc/stat"
	procMemInfoPath = "/proc/meminfo"
)

type Collector struct {
	readFile func(string) ([]byte, error)
	mu       sync.Mutex
	prevCPU  *cpuSample
}

type cpuSample struct {
	total uint64
	idle  uint64
}

func NewCollector() *Collector {
	return &Collector{readFile: os.ReadFile}
}

func (c *Collector) CPUUsagePercent() (float64, bool, error) {
	raw, err := c.readFile(procStatPath)
	if err != nil {
		return 0, false, fmt.Errorf("hostmetrics: read %s: %w", procStatPath, err)
	}

	sample, err := parseCPUSample(string(raw))
	if err != nil {
		return 0, false, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.prevCPU == nil {
		c.prevCPU = &sample
		return 0, false, nil
	}

	deltaTotal := sample.total - c.prevCPU.total
	deltaIdle := sample.idle - c.prevCPU.idle
	c.prevCPU = &sample

	if deltaTotal == 0 || deltaIdle > deltaTotal {
		return 0, false, nil
	}

	usage := 100 * (1 - (float64(deltaIdle) / float64(deltaTotal)))
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}

	return usage, true, nil
}

func (c *Collector) RAMUsagePercent() (float64, bool, error) {
	raw, err := c.readFile(procMemInfoPath)
	if err != nil {
		return 0, false, fmt.Errorf("hostmetrics: read %s: %w", procMemInfoPath, err)
	}

	total, available, ok, err := parseMemInfo(string(raw))
	if err != nil {
		return 0, false, err
	}
	if !ok || total == 0 || available > total {
		return 0, false, nil
	}

	used := total - available
	usage := (float64(used) / float64(total)) * 100
	if usage < 0 {
		usage = 0
	}
	if usage > 100 {
		usage = 100
	}

	return usage, true, nil
}

func parseCPUSample(raw string) (cpuSample, error) {
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 5 {
				return cpuSample{}, fmt.Errorf("hostmetrics: parse %s: missing cpu fields", procStatPath)
			}

			var total uint64
			for i := 1; i < len(fields); i++ {
				v, err := strconv.ParseUint(fields[i], 10, 64)
				if err != nil {
					return cpuSample{}, fmt.Errorf("hostmetrics: parse %s field %d: %w", procStatPath, i, err)
				}
				total += v
			}

			idle, err := strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return cpuSample{}, fmt.Errorf("hostmetrics: parse %s idle: %w", procStatPath, err)
			}
			if len(fields) > 5 {
				iowait, err := strconv.ParseUint(fields[5], 10, 64)
				if err == nil {
					idle += iowait
				}
			}

			return cpuSample{total: total, idle: idle}, nil
		}
	}

	return cpuSample{}, fmt.Errorf("hostmetrics: parse %s: cpu line not found", procStatPath)
}

func parseMemInfo(raw string) (total uint64, available uint64, ok bool, err error) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "MemTotal:":
			v, parseErr := strconv.ParseUint(fields[1], 10, 64)
			if parseErr != nil {
				return 0, 0, false, fmt.Errorf("hostmetrics: parse %s MemTotal: %w", procMemInfoPath, parseErr)
			}
			total = v
		case "MemAvailable:":
			v, parseErr := strconv.ParseUint(fields[1], 10, 64)
			if parseErr != nil {
				return 0, 0, false, fmt.Errorf("hostmetrics: parse %s MemAvailable: %w", procMemInfoPath, parseErr)
			}
			available = v
		}
	}

	if total == 0 || available == 0 {
		return total, available, false, nil
	}

	return total, available, true, nil
}
