package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"astra/pkg/model"
)

func TestParsePSILines(t *testing.T) {
	// Standard Linux 5.8+ /proc/pressure/cpu format
	sample := `some avg10=2.45 avg60=1.12 avg300=0.50 total=1245000
full avg10=0.85 avg60=0.30 avg300=0.10 total=420000
`
	psi, err := ParsePSILines(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("unexpected error parsing PSI: %v", err)
	}

	if psi.SomeAvg10 != 2.45 {
		t.Errorf("expected SomeAvg10=2.45, got %f", psi.SomeAvg10)
	}
	if psi.SomeAvg60 != 1.12 {
		t.Errorf("expected SomeAvg60=1.12, got %f", psi.SomeAvg60)
	}
	if psi.FullAvg10 != 0.85 {
		t.Errorf("expected FullAvg10=0.85, got %f", psi.FullAvg10)
	}
	if psi.FullAvg60 != 0.30 {
		t.Errorf("expected FullAvg60=0.30, got %f", psi.FullAvg60)
	}
}

func TestParseCPUStat(t *testing.T) {
	// Standard cgroups v2 cpu.stat format
	sample := `usage_usec 15200000
user_usec 10100000
system_usec 5100000
nr_periods 1000
nr_throttled 150
throttled_usec 750000
`
	stat, err := ParseCPUStat(strings.NewReader(sample))
	if err != nil {
		t.Fatalf("unexpected error parsing cpu.stat: %v", err)
	}

	if stat.UsageUsec != 15200000 {
		t.Errorf("expected UsageUsec=15200000, got %d", stat.UsageUsec)
	}
	if stat.NrPeriods != 1000 {
		t.Errorf("expected NrPeriods=1000, got %d", stat.NrPeriods)
	}
	if stat.NrThrottled != 150 {
		t.Errorf("expected NrThrottled=150, got %d", stat.NrThrottled)
	}
	if stat.ThrottledRatio != 0.15 {
		t.Errorf("expected ThrottledRatio=0.15, got %f", stat.ThrottledRatio)
	}
}

type mockPSIReader struct {
	val model.PSIPressure
}

func (m *mockPSIReader) ReadPressure(resource string) (model.PSIPressure, error) {
	return m.val, nil
}

type mockCgroupReader struct {
	stat CgroupCPUStat
}

func (m *mockCgroupReader) ReadCPUStat(path string) (CgroupCPUStat, error) {
	return m.stat, nil
}

func (m *mockCgroupReader) ReadMemoryUsage(path string) (uint64, error) {
	return 1024 * 1024 * 1024, nil
}

func TestCollector_CollectOnce(t *testing.T) {
	cfg := Config{
		NodeID:          "test-node-01",
		CollectInterval: 1 * time.Second,
		PSIEnabled:      true,
		CgroupsEnabled:  true,
	}

	psiMock := &mockPSIReader{
		val: model.PSIPressure{SomeAvg10: 12.5, SomeAvg60: 8.0},
	}
	cgMock := &mockCgroupReader{
		stat: CgroupCPUStat{NrPeriods: 100, NrThrottled: 20, ThrottledRatio: 0.20},
	}

	col := NewCollector(cfg, psiMock, cgMock)
	telemetry, err := col.CollectOnce(context.Background(), 850.0)
	if err != nil {
		t.Fatalf("unexpected collection error: %v", err)
	}

	if telemetry.NodeID != "test-node-01" {
		t.Errorf("expected node ID test-node-01, got %s", telemetry.NodeID)
	}
	if telemetry.SequenceNumber != 1 {
		t.Errorf("expected sequence 1, got %d", telemetry.SequenceNumber)
	}
	if telemetry.CPUPressure.SomeAvg10 != 12.5 {
		t.Errorf("expected CPU PSI 12.5, got %f", telemetry.CPUPressure.SomeAvg10)
	}
	if telemetry.CFSThrottledRatio != 0.20 {
		t.Errorf("expected ThrottledRatio 0.20, got %f", telemetry.CFSThrottledRatio)
	}
	if telemetry.RunqueueLatencyP99Us != 850.0 {
		t.Errorf("expected runqueue 850.0 us, got %f", telemetry.RunqueueLatencyP99Us)
	}
}

