package scheduler

import (
	"context"
	"sync"
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

	nodeA := model.NewNode("node-a", 4000, 16*1024*1024*1024)
	nodeB := model.NewNode("node-b", 4000, 16*1024*1024*1024)

	batchW := &model.Workload{
		ID:                 "batch-worker",
		Class:              model.ClassThroughputBatch,
		CPURequestMillis:   1000,
		MemoryRequestBytes: 2 * 1024 * 1024 * 1024,
	}
	_ = nodeA.AssignWorkload(batchW)

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
	node.LastTelemetry = &model.NodeTelemetry{
		NodeID:      "node-1",
		CollectedAt: now.Add(-10 * time.Second),
		CPUPressure: model.PSIPressure{SomeAvg10: 95.0},
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

func TestEngine_AdversarialInputsAndEdgeCases(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)
	now := time.Now()

	// 1. Nil workload
	if _, err := engine.Schedule(context.Background(), nil, map[string]*model.Node{"n": model.NewNode("n", 1000, 1000)}, now); err != ErrNilWorkload {
		t.Fatalf("expected ErrNilWorkload, got: %v", err)
	}

	// 2. Empty nodes map
	w := &model.Workload{
		ID:                 "w1",
		Class:              model.ClassLatencySensitive,
		CPURequestMillis:   500,
		MemoryRequestBytes: 1024,
	}
	if _, err := engine.Schedule(context.Background(), w, map[string]*model.Node{}, now); err != ErrNoRegisteredNodes {
		t.Fatalf("expected ErrNoRegisteredNodes, got: %v", err)
	}

	// 3. Map containing nil Node pointers and nodes with zero/negative capacity
	nodesWithBadData := map[string]*model.Node{
		"nil-node":  nil,
		"zero-node": model.NewNode("zero-node", 0, 0),
		"good-node": model.NewNode("good-node", 4000, 4096*1024*1024),
	}
	decision, err := engine.Schedule(context.Background(), w, nodesWithBadData, now)
	if err != nil {
		t.Fatalf("unexpected error with bad nodes present: %v", err)
	}
	if decision.SelectedNodeID != "good-node" {
		t.Fatalf("expected good-node to be selected, got: %s", decision.SelectedNodeID)
	}

	// 4. Context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := engine.Schedule(ctx, w, nodesWithBadData, now); err != context.Canceled {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}

	// 5. Future clock-skew telemetry
	goodNode := nodesWithBadData["good-node"]
	goodNode.LastTelemetry = &model.NodeTelemetry{
		NodeID:      "good-node",
		CollectedAt: now.Add(2 * time.Hour), // Future clock skew
	}
	skewDecision, err := engine.Schedule(context.Background(), w, map[string]*model.Node{"good-node": goodNode}, now)
	if err != nil {
		t.Fatalf("unexpected schedule error: %v", err)
	}
	if !skewDecision.TelemetryDegraded {
		t.Fatalf("expected clock-skewed telemetry to be degraded gracefully")
	}
}

func TestEngine_ConcurrencyStress(t *testing.T) {
	engine := NewEngine(DefaultConfig(), nil)
	now := time.Now()

	nodes := make(map[string]*model.Node)
	for i := 0; i < 20; i++ {
		id := string(rune('A' + i))
		nodes[id] = model.NewNode(id, 8000, 32*1024*1024*1024)
	}

	var wg sync.WaitGroup
	workers := 50
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := &model.Workload{
				ID:                 string(rune('w' + idx)),
				Class:              model.ClassLatencySensitive,
				CPURequestMillis:   500,
				MemoryRequestBytes: 1024 * 1024 * 1024,
			}
			dec, err := engine.Schedule(context.Background(), w, nodes, now)
			if err != nil || dec == nil {
				t.Errorf("worker %d failed schedule: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()
}
