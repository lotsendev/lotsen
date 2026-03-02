package hostinfo

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const osReleasePath = "/etc/os-release"

type Metadata struct {
	IPAddress   string
	OSName      string
	OSVersion   string
	CPUCores    int
	MemoryBytes uint64
	DiskBytes   uint64
}

func Collect() Metadata {
	metadata := Metadata{CPUCores: runtime.NumCPU()}

	if ip := primaryIPv4(); ip != "" {
		metadata.IPAddress = ip
	}

	if name, version := osIdentity(); name != "" || version != "" {
		metadata.OSName = name
		metadata.OSVersion = version
	}

	if memoryBytes, ok := totalMemoryBytes(); ok {
		metadata.MemoryBytes = memoryBytes
	}

	if diskBytes, ok := totalDiskBytes("/"); ok {
		metadata.DiskBytes = diskBytes
	}

	return metadata
}

func primaryIPv4() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return ""
	}

	ip := addr.IP.To4()
	if ip == nil {
		return ""
	}

	return ip.String()
}

func osIdentity() (string, string) {
	f, err := os.Open(osReleasePath)
	if err != nil {
		return "", ""
	}
	defer f.Close()

	name := ""
	version := ""
	prettyName := ""

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := strings.Trim(parts[1], "\"")

		switch key {
		case "PRETTY_NAME":
			prettyName = value
		case "NAME":
			name = value
		case "VERSION_ID":
			version = value
		}
	}

	if prettyName != "" {
		return prettyName, version
	}

	return name, version
}

func totalMemoryBytes() (uint64, bool) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}

		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}

		return kib * 1024, true
	}

	return 0, false
}

func totalDiskBytes(path string) (uint64, bool) {
	var stats syscall.Statfs_t
	if err := syscall.Statfs(path, &stats); err != nil {
		return 0, false
	}

	return stats.Blocks * uint64(stats.Bsize), true
}
