package model

import (
	"sync"
	"testing"
	"time"
)

func TestWorkload_Validation(t *testing.T) {
	// 1. Valid workload
	wValid := &Workload{
		ID:                 "valid-1",
		Class:              ClassLatencySensitive,
		CPURequestMillis:   500,
		MemoryRequestBytes: 1024 * 1024 * 512,
		SLO:                &SLOSpec{TargetP99Ms: 25.0, MaxAllowedJitterMs: 5.0},
	}
	if err := wValid.Validate(); err != nil {
		t.Fatalf("expected valid workload to pass, got: %v", err)
	}

	// 2. Empty ID
	wNoID := &Workload{
		ID:                 "",
		Class:              ClassThroughputBatch,
		CPURequestMillis:   1000,
		MemoryRequestBytes: 1024,
	}
	if err := wNoID.Validate(); err == nil {
		t.Fatalf("expected error for empty workload ID")
	}

	// 3. Zero / Negative CPU
	wNegCPU := &Workload{
		ID:                 "neg-cpu",
		Class:              ClassThroughputBatch,
		CPURequestMillis:   -100,
		MemoryRequestBytes: 1024,
	}
	if err := wNegCPU.Validate(); err == nil {
		t.Fatalf("expected error for negative CPU request")
	}

	// 4. Invalid SLO
	wBadSLO := &Workload{
		ID:                 "bad-slo",
		Class:              ClassLatencySensitive,
		CPURequestMillis:   500,
		MemoryRequestBytes: 1024,
		SLO:                &SLOSpec{TargetP99Ms: -5.0},
	}
	if err := wBadSLO.Validate(); err == nil {
		t.Fatalf("expected error for negative SLO target")
	}
}

func TestNode_ThreadSafeAssignmentAndDuplicateGuard(t *testing.T) {
	node := NewNode("node-concur", 8000, 16*1024*1024*1024)

	w := &Workload{
		ID:                 "task-1",
		Class:              ClassThroughputBatch,
		CPURequestMillis:   1000,
		MemoryRequestBytes: 1024 * 1024 * 1024,
	}

	// First assignment succeeds
	if err := node.AssignWorkload(w); err != nil {
		t.Fatalf("first assignment failed: %v", err)
	}

	// Duplicate assignment MUST fail and NOT leak resources
	if err := node.AssignWorkload(w); err == nil {
		t.Fatalf("expected duplicate assignment to fail")
	}

	if node.AllocatedCPU != 1000 {
		t.Fatalf("expected AllocatedCPU to remain 1000 after duplicate reject, got %d", node.AllocatedCPU)
	}

	// Removal restores capacity
	node.RemoveWorkload("task-1")
	if node.AllocatedCPU != 0 || node.AllocatedMemory != 0 {
		t.Fatalf("expected 0 allocated after removal, got cpu=%d mem=%d", node.AllocatedCPU, node.AllocatedMemory)
	}

	// Concurrent assignment test
	var wg sync.WaitGroup
	for i := 0; i < 7; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cw := &Workload{
				ID:                 string(rune('a' + idx)),
				Class:              ClassLatencySensitive,
				CPURequestMillis:   1000,
				MemoryRequestBytes: 1024 * 1024 * 1024,
			}
			_ = node.AssignWorkload(cw)
		}(i)
	}
	wg.Wait()

	if node.AllocatedCPU > 8000 {
		t.Fatalf("allocated CPU exceeded capacity under concurrency: %d", node.AllocatedCPU)
	}
}

func TestNodeTelemetry_StalenessAndClockSkew(t *testing.T) {
	now := time.Now()
	maxAge := 5 * time.Second

	// 1. Fresh telemetry
	fresh := &NodeTelemetry{
		NodeID:      "node-1",
		CollectedAt: now.Add(-1 * time.Second),
	}
	if fresh.IsStale(now, maxAge) {
		t.Fatalf("fresh telemetry should not be stale")
	}

	// 2. Stale telemetry (older than maxAge)
	old := &NodeTelemetry{
		NodeID:      "node-1",
		CollectedAt: now.Add(-10 * time.Second),
	}
	if !old.IsStale(now, maxAge) {
		t.Fatalf("old telemetry must be marked stale")
	}

	// 3. Clock skew: Future timestamp (adversarial or NTP drift)
	future := &NodeTelemetry{
		NodeID:      "node-1",
		CollectedAt: now.Add(1 * time.Hour),
	}
	if !future.IsStale(now, maxAge) {
		t.Fatalf("future timestamp due to clock skew must be rejected as stale")
	}

	// 4. Nil or zero timestamp
	if !(*NodeTelemetry)(nil).IsStale(now, maxAge) {
		t.Fatalf("nil telemetry must be stale")
	}
	zero := &NodeTelemetry{}
	if !zero.IsStale(now, maxAge) {
		t.Fatalf("zero-value telemetry must be stale")
	}
}

