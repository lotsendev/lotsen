package hostmetrics

import (
	"errors"
	"fmt"
	"testing"
)

func TestCollector_CPUUsagePercent(t *testing.T) {
	reads := map[string]string{
		procStatPath: "cpu  100 0 100 700 100 0 0 0 0 0\n",
	}

	collector := &Collector{
		readFile: func(path string) ([]byte, error) {
			v, ok := reads[path]
			if !ok {
				return nil, errors.New("not found")
			}
			return []byte(v), nil
		},
	}

	_, ok, err := collector.CPUUsagePercent()
	if err != nil {
		t.Fatalf("first sample: %v", err)
	}
	if ok {
		t.Fatal("first sample should be unavailable")
	}

	reads[procStatPath] = "cpu  200 0 150 750 100 0 0 0 0 0\n"

	usage, ok, err := collector.CPUUsagePercent()
	if err != nil {
		t.Fatalf("second sample: %v", err)
	}
	if !ok {
		t.Fatal("second sample should be available")
	}

	if usage != 75 {
		t.Fatalf("want cpu usage 75, got %v", usage)
	}
}

func TestCollector_RAMUsagePercent(t *testing.T) {
	collector := &Collector{
		readFile: func(path string) ([]byte, error) {
			if path != procMemInfoPath {
				return nil, errors.New("not found")
			}
			return []byte("MemTotal:       1000000 kB\nMemAvailable:    250000 kB\n"), nil
		},
	}

	usage, ok, err := collector.RAMUsagePercent()
	if err != nil {
		t.Fatalf("RAMUsagePercent: %v", err)
	}
	if !ok {
		t.Fatal("ram sample should be available")
	}
	if usage != 75 {
		t.Fatalf("want ram usage 75, got %v", usage)
	}
}

func TestCollector_RAMUsagePercent_UnavailableWhenMemAvailableMissing(t *testing.T) {
	collector := &Collector{
		readFile: func(path string) ([]byte, error) {
			if path != procMemInfoPath {
				return nil, errors.New("not found")
			}
			return []byte("MemTotal:       1000000 kB\n"), nil
		},
	}

	_, ok, err := collector.RAMUsagePercent()
	if err != nil {
		t.Fatalf("RAMUsagePercent: %v", err)
	}
	if ok {
		t.Fatal("ram sample should be unavailable when MemAvailable is missing")
	}
}

func TestParseCPUSample_ErrorWhenMissingCPU(t *testing.T) {
	_, err := parseCPUSample("intr 1\n")
	if err == nil {
		t.Fatal("want parse error when cpu line is missing")
	}
}

func TestParseMemInfo_ParseError(t *testing.T) {
	_, _, _, err := parseMemInfo("MemTotal: oops kB\nMemAvailable: 10 kB\n")
	if err == nil {
		t.Fatal("want parse error")
	}
	if got := err.Error(); got == "" {
		t.Fatalf("want non-empty error, got %q", got)
	}
}

func TestCollector_CPUUsagePercent_ReadError(t *testing.T) {
	collector := &Collector{
		readFile: func(path string) ([]byte, error) {
			return nil, fmt.Errorf("read failed: %s", path)
		},
	}

	_, _, err := collector.CPUUsagePercent()
	if err == nil {
		t.Fatal("want read error")
	}
}
