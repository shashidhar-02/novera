// Package agent provides Linux node telemetry ingestion from cgroups v2 and Pressure Stall Information (PSI).
package agent

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"astra/pkg/model"
)

// PSIReader defines the interface for reading Linux Pressure Stall Information.
type PSIReader interface {
	ReadPressure(resource string) (model.PSIPressure, error)
}

// OSPSIReader reads PSI metrics from the real Linux /proc/pressure virtual filesystem.
type OSPSIReader struct {
	basePath string
}

// NewOSPSIReader initializes a reader targeting /proc/pressure or custom path for testing.
func NewOSPSIReader(basePath string) *OSPSIReader {
	if basePath == "" {
		basePath = "/proc/pressure"
	}
	return &OSPSIReader{basePath: basePath}
}

// ReadPressure reads and parses /proc/pressure/{cpu,memory,io}.
func (r *OSPSIReader) ReadPressure(resource string) (model.PSIPressure, error) {
	filePath := fmt.Sprintf("%s/%s", r.basePath, resource)
	f, err := os.Open(filePath)
	if err != nil {
		return model.PSIPressure{}, fmt.Errorf("failed to open PSI path %s: %w", filePath, err)
	}
	defer f.Close()

	return ParsePSILines(f)
}

// ParsePSILines parses standard Linux kernel PSI output formatted as:
// some avg10=0.00 avg60=0.00 avg300=0.00 total=0
// full avg10=0.00 avg60=0.00 avg300=0.00 total=0
func ParsePSILines(r io.Reader) (model.PSIPressure, error) {
	scanner := bufio.NewScanner(r)
	var psi model.PSIPressure

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		lineType := fields[0] // "some" or "full"
		for _, kv := range fields[1:] {
			parts := strings.Split(kv, "=")
			if len(parts) != 2 {
				continue
			}
			val, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				continue
			}

			switch lineType {
			case "some":
				switch parts[0] {
				case "avg10":
					psi.SomeAvg10 = val
				case "avg60":
					psi.SomeAvg60 = val
				}
			case "full":
				switch parts[0] {
				case "avg10":
					psi.FullAvg10 = val
				case "avg60":
					psi.FullAvg60 = val
				}
			}
		}
	}

	return psi, scanner.Err()
}
