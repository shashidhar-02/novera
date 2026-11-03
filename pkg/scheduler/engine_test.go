package scheduler

import (
	"context"
	"testing"
	"time"

	"astra/pkg/model"
)

func TestEngine_Filtering(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)
	now := time.Now()

	nodes := map[string]*model.Node{
		"small-node": model.NewNode("small-node", 1000, 1024*1024*1024), // 1 core, 1 GB
		"large-node": model.NewNode("large-node", 8000, 8192*1024*1024), // 8 cores, 8 GB
	}

	// Workload requiring 2 cores (2000m)
	w := &model.Workload{
		ID:                 "large-workload",
		Class:              model.ClassThroughputBatch,
		CPURequestMillis:   2000,
		MemoryRequestBytes: 1024 * 1024 * 1024,
	}

	decision, err := engine.Schedule(context.Background(), w, nodes, now)
	if err != nil {
		t.Fatalf("expected successful schedule, got error: %v", err)
	}

	if decision.SelectedNodeID != "large-node" {
		t.Fatalf("expected placement on large-node, got %s", decision.SelectedNodeID)
	}

	if _, ok := decision.FilteredNodes["small-node"]; !ok {
		t.Fatalf("expected small-node to be filtered due to insufficient CPU")
	}
}

func TestEngine_InterferenceSteering(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)
	now := time.Now()

	// Two nodes with identical capacity (4 cores, 16 GB)
	nodeA := model.NewNode("node-a", 4000, 16*1024*1024*1024)
	nodeB := model.NewNode("node-b", 4000, 16*1024*1024*1024)

	// Node A already runs a heavy batch workload
	batchW := &model.Workload{
		ID:                 "batch-worker",
		Class:              model.ClassThroughputBatch,
		CPURequestMillis:   1000,
		MemoryRequestBytes: 2 * 1024 * 1024 * 1024,
	}
	_ = nodeA.AssignWorkload(batchW)

	// Fresh Latency-Sensitive workload arriving
	lsW := &model.Workload{
		ID:                 "api-gateway",
		Class:              model.ClassLatencySensitive,
		CPURequestMillis:   1000,
		MemoryRequestBytes: 2 * 1024 * 1024 * 1024,
	}

	nodes := map[string]*model.Node{
		"node-a": nodeA,
		"node-b": nodeB,
	}

	decision, err := engine.Schedule(context.Background(), lsW, nodes, now)
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	// Engine should avoid Node A because of Batch interference penalty, selecting clean Node B
	if decision.SelectedNodeID != "node-b" {
		t.Fatalf("expected interference-aware steering to select clean node-b, selected: %s", decision.SelectedNodeID)
	}
}

func TestEngine_StaleTelemetryFallback(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxTelemetryAge = 2 * time.Second
	engine := NewEngine(cfg, nil)

	now := time.Now()
	node := model.NewNode("node-1", 4000, 8*1024*1024*1024)
	// Stale telemetry from 10 seconds ago
	node.LastTelemetry = &model.NodeTelemetry{
		NodeID:      "node-1",
		CollectedAt: now.Add(-10 * time.Second),
		CPUPressure: model.PSIPressure{SomeAvg10: 95.0}, // severe historical pressure
	}

	w := &model.Workload{
		ID:                 "test-w",
		Class:              model.ClassLatencySensitive,
		CPURequestMillis:   500,
		MemoryRequestBytes: 512 * 1024 * 1024,
	}

	decision, err := engine.Schedule(context.Background(), w, map[string]*model.Node{"node-1": node}, now)
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}

	if !decision.TelemetryDegraded {
		t.Fatalf("expected TelemetryDegraded flag to be true when telemetry is older than max age")
	}
}
