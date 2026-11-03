package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// CgroupCPUStat represents CPU throttling metrics from cgroups v2 cpu.stat.
type CgroupCPUStat struct {
	UsageUsec     uint64  `json:"usage_usec"`
	UserUsec      uint64  `json:"user_usec"`
	SystemUsec    uint64  `json:"system_usec"`
	NrPeriods     uint64  `json:"nr_periods"`
	NrThrottled   uint64  `json:"nr_throttled"`
	ThrottledUsec uint64  `json:"throttled_usec"`
	ThrottledRatio float64 `json:"throttled_ratio"` // nr_throttled / nr_periods
}

// CgroupReader abstracts cgroups v2 metric ingestion.
type CgroupReader interface {
	ReadCPUStat(cgroupPath string) (CgroupCPUStat, error)
	ReadMemoryUsage(cgroupPath string) (uint64, error)
}

// OSCgroupReader reads directly from Linux /sys/fs/cgroup.
type OSCgroupReader struct {
	basePath string
}

// NewOSCgroupReader initializes a reader targeting /sys/fs/cgroup or a mock path.
func NewOSCgroupReader(basePath string) *OSCgroupReader {
	if basePath == "" {
		basePath = "/sys/fs/cgroup"
	}
	return &OSCgroupReader{basePath: basePath}
}

// ReadCPUStat reads /sys/fs/cgroup/<path>/cpu.stat.
func (r *OSCgroupReader) ReadCPUStat(cgroupPath string) (CgroupCPUStat, error) {
	fullPath := fmt.Sprintf("%s/%s/cpu.stat", r.basePath, strings.TrimPrefix(cgroupPath, "/"))
	f, err := os.Open(fullPath)
	if err != nil {
		return CgroupCPUStat{}, fmt.Errorf("failed to open cgroup cpu.stat at %s: %w", fullPath, err)
	}
	defer f.Close()

	return ParseCPUStat(f)
}

// ReadMemoryUsage reads /sys/fs/cgroup/<path>/memory.current.
func (r *OSCgroupReader) ReadMemoryUsage(cgroupPath string) (uint64, error) {
	fullPath := fmt.Sprintf("%s/%s/memory.current", r.basePath, strings.TrimPrefix(cgroupPath, "/"))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open cgroup memory.current at %s: %w", fullPath, err)
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory.current value: %w", err)
	}
	return val, nil
}

// ParseCPUStat parses key-value lines from Linux cgroups v2 cpu.stat.
func ParseCPUStat(r io.Reader) (CgroupCPUStat, error) {
	scanner := bufio.NewScanner(r)
	var stat CgroupCPUStat

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		val, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			continue
		}

		switch parts[0] {
		case "usage_usec":
			stat.UsageUsec = val
		case "user_usec":
			stat.UserUsec = val
		case "system_usec":
			stat.SystemUsec = val
		case "nr_periods":
			stat.NrPeriods = val
		case "nr_throttled":
			stat.NrThrottled = val
		case "throttled_usec":
			stat.ThrottledUsec = val
		}
	}

	if stat.NrPeriods > 0 {
		stat.ThrottledRatio = float64(stat.NrThrottled) / float64(stat.NrPeriods)
		if stat.ThrottledRatio > 1.0 {
			stat.ThrottledRatio = 1.0
		}
	}

	return stat, scanner.Err()
}
