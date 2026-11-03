// Package interference models runtime contention dynamics between co-located workload classes.
package interference

import (
	"astra/pkg/model"
)

// Model calculates interference risk scores between an incoming workload and existing node workloads.
type Model interface {
	// Evaluate calculates an interference penalty score [0.0, 100.0] and returns explanation reasons.
	Evaluate(incoming *model.Workload, node *model.Node) (penalty float64, reasons []string)
}

// MatrixModel implements an empirical cross-class contention penalty matrix.
type MatrixModel struct {
	// pairwisePenalties maps [IncomingClass][ExistingClass] -> penalty points
	pairwisePenalties map[model.WorkloadClass]map[model.WorkloadClass]float64
}

// NewDefaultMatrixModel initializes empirical contention weights derived from systems research.
func NewDefaultMatrixModel() *MatrixModel {
	penalties := make(map[model.WorkloadClass]map[model.WorkloadClass]float64)

	// Latency-sensitive workloads are highly vulnerable to CPU runqueue stalls from Batch
	// and I/O buffer thrashing from IO-intensive tasks.
	penalties[model.ClassLatencySensitive] = map[model.WorkloadClass]float64{
		model.ClassLatencySensitive: 5.0,  // Mild L3 cache contention
		model.ClassThroughputBatch:  35.0, // Severe CPU runqueue and context-switch interference
		model.ClassIOIntensive:      25.0, // High page-cache and I/O queue interference
	}

	// Throughput batch jobs are relatively robust, but IO-intensive jobs may starve memory bandwidth
	penalties[model.ClassThroughputBatch] = map[model.WorkloadClass]float64{
		model.ClassLatencySensitive: 5.0,
		model.ClassThroughputBatch:  0.0,  // Batch workloads peacefully co-exist under CFS fair sharing
		model.ClassIOIntensive:      15.0, // Minor I/O wait contention
	}

	// IO-intensive workloads
	penalties[model.ClassIOIntensive] = map[model.WorkloadClass]float64{
		model.ClassLatencySensitive: 10.0,
		model.ClassThroughputBatch:  15.0,
		model.ClassIOIntensive:      30.0, // Severe disk/network queue head-of-line blocking
	}

	return &MatrixModel{pairwisePenalties: penalties}
}

// Evaluate computes the aggregate interference score and diagnostic reasons.
// It is fully nil-safe and handles malformed inputs gracefully.
func (m *MatrixModel) Evaluate(incoming *model.Workload, node *model.Node) (float64, []string) {
	if incoming == nil {
		return 0.0, []string{"nil incoming workload: zero penalty applied"}
	}
	if node == nil {
		return 0.0, []string{"nil candidate node: zero penalty applied"}
	}

	activeWorkloads := node.GetActiveWorkloads()
	if len(activeWorkloads) == 0 {
		return 0.0, []string{"empty node: zero interference"}
	}

	totalPenalty := 0.0
	var reasons []string

	classMap, exists := m.pairwisePenalties[incoming.Class]
	if !exists {
		return 0.0, []string{"unrecognized workload class: zero penalty applied"}
	}

	// Count active workload classes on this node using thread-safe snapshot
	activeClassCounts := make(map[model.WorkloadClass]int)
	for _, active := range activeWorkloads {
		if active != nil {
			activeClassCounts[active.Class]++
		}
	}

	for class, count := range activeClassCounts {
		if penaltyPerTask, ok := classMap[class]; ok && penaltyPerTask > 0 {
			classPenalty := penaltyPerTask * float64(count)
			if classPenalty > 60.0 {
				classPenalty = 60.0 // Cap individual class contribution
			}
			totalPenalty += classPenalty

			if incoming.Class == model.ClassLatencySensitive && class == model.ClassThroughputBatch {
				reasons = append(reasons, "high contention risk: co-locating LatencySensitive with ThroughputBatch")
			} else if incoming.Class == model.ClassLatencySensitive && class == model.ClassIOIntensive {
				reasons = append(reasons, "I/O stall risk: co-locating LatencySensitive with IOIntensive")
			} else if incoming.Class == model.ClassIOIntensive && class == model.ClassIOIntensive {
				reasons = append(reasons, "disk queue contention: multiple IOIntensive tasks")
			}
		}
	}

	// Cap total penalty to 100.0
	if totalPenalty > 100.0 {
		totalPenalty = 100.0
	}

	return totalPenalty, reasons
}
