package interference

import (
	"testing"

	"astra/pkg/model"
)

func TestMatrixModel_Evaluate(t *testing.T) {
	m := NewDefaultMatrixModel()

	node := model.NewNode("test-node", 8000, 32*1024*1024*1024)

	// 1. Empty node evaluation
	incomingLS := &model.Workload{
		ID:    "ls-1",
		Class: model.ClassLatencySensitive,
	}
	penalty, reasons := m.Evaluate(incomingLS, node)
	if penalty != 0.0 {
		t.Fatalf("expected 0.0 penalty for empty node, got %f", penalty)
	}
	if len(reasons) == 0 {
		t.Errorf("expected diagnostic reasons for empty node")
	}

	// 2. Add ThroughputBatch workload to node
	batchJob := &model.Workload{
		ID:    "batch-1",
		Class: model.ClassThroughputBatch,
	}
	node.ActiveWorkloads["batch-1"] = batchJob

	penaltyWithBatch, reasonsWithBatch := m.Evaluate(incomingLS, node)
	if penaltyWithBatch < 30.0 {
		t.Fatalf("expected high penalty for LS incoming onto Batch node, got %f", penaltyWithBatch)
	}
	foundReason := false
	for _, r := range reasonsWithBatch {
		if r == "high contention risk: co-locating LatencySensitive with ThroughputBatch" {
			foundReason = true
			break
		}
	}
	if !foundReason {
		t.Errorf("expected contention warning in reasons: %v", reasonsWithBatch)
	}

	// 3. Batch incoming onto Batch node should have 0 penalty
	incomingBatch := &model.Workload{
		ID:    "batch-2",
		Class: model.ClassThroughputBatch,
	}
	batchPenalty, _ := m.Evaluate(incomingBatch, node)
	if batchPenalty != 0.0 {
		t.Fatalf("expected 0.0 penalty for Batch co-locating with Batch, got %f", batchPenalty)
	}
}
