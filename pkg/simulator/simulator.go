// Package simulator implements a deterministic multi-node cluster simulator for comparing scheduling policies.
package simulator

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"astra/pkg/model"
	"astra/pkg/scheduler"
)

// Policy designates which scheduling logic to use during the simulation run.
type Policy string

const (
	// PolicyBaseline uses standard round-robin / static capacity balancing without interference awareness.
	PolicyBaseline Policy = "Baseline-StaticCapacity"

	// PolicyAstra uses ASTRA's kernel-pressure and interference-aware scoring.
	PolicyAstra Policy = "ASTRA-InterferenceAware"
)

// ScenarioConfig configures the simulation run.
type ScenarioConfig struct {
	Seed              int64         `json:"seed"`
	NodeCount         int           `json:"node_count"`
	NodeCPUCores      int64         `json:"node_cpu_cores"`     // Cores per node (e.g., 8)
	NodeMemoryGB      int64         `json:"node_memory_gb"`     // GB per node (e.g., 32)
	WorkloadCount     int           `json:"workload_count"`     // Total workloads to schedule
	RatioLatencySens  float64       `json:"ratio_latency_sens"` // e.g. 0.40
	RatioBatch        float64       `json:"ratio_batch"`        // e.g. 0.35
	RatioIOIntensive  float64       `json:"ratio_io_intensive"` // e.g. 0.25
	TelemetryInterval time.Duration `json:"telemetry_interval"`
}

// DefaultScenario returns a standardized benchmark scenario.
func DefaultScenario(seed int64) ScenarioConfig {
	return ScenarioConfig{
		Seed:              seed,
		NodeCount:         6,
		NodeCPUCores:      8,  // 8000m CPU
		NodeMemoryGB:      32, // 32768 MB
		WorkloadCount:     40,
		RatioLatencySens:  0.40,
		RatioBatch:        0.35,
		RatioIOIntensive:  0.25,
		TelemetryInterval: 1 * time.Second,
	}
}

// MetricsReport captures comparative metrics from a completed simulation run.
type MetricsReport struct {
	Policy                Policy  `json:"policy"`
	Seed                  int64   `json:"seed"`
	TotalWorkloads        int     `json:"total_workloads"`
	PlacedWorkloads       int     `json:"placed_workloads"`
	RejectedWorkloads     int     `json:"rejected_workloads"`
	LatencySensWorkloads  int     `json:"latency_sens_workloads"`
	SLOViolations         int     `json:"slo_violations"`
	SLOViolationRate      float64 `json:"slo_violation_rate"`       // %
	AvgNodeCPUUtilization float64 `json:"avg_node_cpu_utilization"` // %
	AvgNodeMemUtilization float64 `json:"avg_node_mem_utilization"` // %
	AvgRunqueueLatencyUs  float64 `json:"avg_runqueue_latency_us"`
	MaxRunqueueLatencyUs  float64 `json:"max_runqueue_latency_us"`
}

// ClusterSimulator manages the deterministic environment.
type ClusterSimulator struct {
	cfg       ScenarioConfig
	rnd       *rand.Rand
	nodes     map[string]*model.Node
	workloads []*model.Workload
}

// NewClusterSimulator initializes nodes and generates synthetic workloads deterministically from seed.
func NewClusterSimulator(cfg ScenarioConfig) *ClusterSimulator {
	rnd := rand.New(rand.NewSource(cfg.Seed))
	nodes := make(map[string]*model.Node)

	for i := 0; i < cfg.NodeCount; i++ {
		id := fmt.Sprintf("node-%02d", i+1)
		nodes[id] = model.NewNode(id, cfg.NodeCPUCores*1000, cfg.NodeMemoryGB*1024*1024*1024)
	}

	workloads := make([]*model.Workload, 0, cfg.WorkloadCount)
	for i := 0; i < cfg.WorkloadCount; i++ {
		roll := rnd.Float64()
		var class model.WorkloadClass
		var cpuMillis int64
		var memBytes int64
		var slo *model.SLOSpec

		if roll < cfg.RatioLatencySens {
			class = model.ClassLatencySensitive
			cpuMillis = 500 + int64(rnd.Intn(500))            // 500 - 1000m
			memBytes = int64(512+rnd.Intn(512)) * 1024 * 1024 // 512 - 1024 MB
			slo = &model.SLOSpec{TargetP99Ms: 25.0 + float64(rnd.Intn(15))}
		} else if roll < cfg.RatioLatencySens+cfg.RatioBatch {
			class = model.ClassThroughputBatch
			cpuMillis = 1500 + int64(rnd.Intn(1500))            // 1500 - 3000m
			memBytes = int64(2048+rnd.Intn(2048)) * 1024 * 1024 // 2 - 4 GB
		} else {
			class = model.ClassIOIntensive
			cpuMillis = 1000 + int64(rnd.Intn(1000))            // 1000 - 2000m
			memBytes = int64(1024+rnd.Intn(2048)) * 1024 * 1024 // 1 - 3 GB
		}

		w := &model.Workload{
			ID:                 fmt.Sprintf("wl-%03d-%s", i+1, class),
			Namespace:          "default",
			Class:              class,
			CPURequestMillis:   cpuMillis,
			MemoryRequestBytes: memBytes,
			SLO:                slo,
		}
		workloads = append(workloads, w)
	}

	return &ClusterSimulator{
		cfg:       cfg,
		rnd:       rnd,
		nodes:     nodes,
		workloads: workloads,
	}
}

// Reset clears node allocations.
func (cs *ClusterSimulator) Reset() {
	for _, n := range cs.nodes {
		n.AllocatedCPU = 0
		n.AllocatedMemory = 0
		n.ActiveWorkloads = make(map[string]*model.Workload)
		n.LastTelemetry = nil
	}
}

// Run executes the simulation under the specified policy.
func (cs *ClusterSimulator) Run(policy Policy) *MetricsReport {
	cs.Reset()
	currentTime := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	// Engine for ASTRA
	astraEngine := scheduler.NewEngine(scheduler.DefaultConfig(), nil)

	placedCount := 0
	rejectedCount := 0
	lsCount := 0

	// Round-robin index for baseline
	nodeIDs := make([]string, 0, len(cs.nodes))
	for id := range cs.nodes {
		nodeIDs = append(nodeIDs, id)
	}

	for _, w := range cs.workloads {
		if w.Class == model.ClassLatencySensitive {
			lsCount++
		}

		currentTime = currentTime.Add(cs.cfg.TelemetryInterval)
		cs.updateSyntheticTelemetry(currentTime)

		var targetNodeID string

		if policy == PolicyBaseline {
			// Baseline: simple first-fit / least-allocated without interference or PSI awareness
			bestFitID := ""
			lowestUtil := 1.01
			for _, id := range nodeIDs {
				n := cs.nodes[id]
				if n.AvailableCPU() >= w.CPURequestMillis && n.AvailableMemory() >= w.MemoryRequestBytes {
					util := float64(n.AllocatedCPU) / float64(n.CPUCapacityMillis)
					if util < lowestUtil {
						lowestUtil = util
						bestFitID = id
					}
				}
			}
			targetNodeID = bestFitID
		} else {
			// ASTRA interference-aware scheduler
			decision, err := astraEngine.Schedule(context.Background(), w, cs.nodes, currentTime)
			if err == nil && decision != nil {
				targetNodeID = decision.SelectedNodeID
			}
		}

		if targetNodeID == "" {
			rejectedCount++
			continue
		}

		// Bind workload to selected node
		err := cs.nodes[targetNodeID].AssignWorkload(w)
		if err != nil {
			rejectedCount++
		} else {
			placedCount++
		}
	}

	// Final telemetry refresh and evaluation
	currentTime = currentTime.Add(cs.cfg.TelemetryInterval)
	cs.updateSyntheticTelemetry(currentTime)

	return cs.evaluatePerformance(policy, placedCount, rejectedCount, lsCount)
}

// updateSyntheticTelemetry computes realistic PSI and runqueue latencies based on co-located workloads.
func (cs *ClusterSimulator) updateSyntheticTelemetry(now time.Time) {
	for _, n := range cs.nodes {
		batchCount := 0
		lsCount := 0
		ioCount := 0

		for _, w := range n.ActiveWorkloads {
			switch w.Class {
			case model.ClassThroughputBatch:
				batchCount++
			case model.ClassLatencySensitive:
				lsCount++
			case model.ClassIOIntensive:
				ioCount++
			}
		}

		cpuUtil := float64(n.AllocatedCPU) / float64(n.CPUCapacityMillis)
		memUtil := float64(n.AllocatedMemory) / float64(n.MemoryCapacityBytes)

		// Model non-linear interference stalls
		// If batch jobs co-exist with latency-sensitive tasks, runqueue latency escalates rapidly
		runqBaseUs := 150.0 // Baseline kernel scheduling latency
		if batchCount > 0 && lsCount > 0 {
			// Significant context-switch and CFS queue thrashing
			runqBaseUs += float64(batchCount*lsCount) * 850.0
		}
		if cpuUtil > 0.70 {
			runqBaseUs += math.Pow((cpuUtil-0.70)*10.0, 2) * 500.0
		}

		// CPU PSI: some_avg10 percentage
		cpuPSI := cpuUtil * 12.0
		if batchCount > 1 {
			cpuPSI += float64(batchCount) * 3.5
		}

		// Memory PSI
		memPSI := memUtil * 8.0
		if memUtil > 0.85 {
			memPSI += 25.0
		}

		// IO PSI
		ioPSI := float64(ioCount) * 6.0

		n.LastTelemetry = &model.NodeTelemetry{
			NodeID:      n.ID,
			CollectedAt: now,
			CPUPressure: model.PSIPressure{
				SomeAvg10: math.Min(100.0, cpuPSI),
				SomeAvg60: math.Min(100.0, cpuPSI*0.8),
			},
			MemoryPressure: model.PSIPressure{
				SomeAvg10: math.Min(100.0, memPSI),
				SomeAvg60: math.Min(100.0, memPSI*0.8),
			},
			IOPressure: model.PSIPressure{
				SomeAvg10: math.Min(100.0, ioPSI),
				SomeAvg60: math.Min(100.0, ioPSI*0.8),
			},
			RunqueueLatencyP99Us: math.Min(50000.0, runqBaseUs),
			CFSThrottledRatio:    math.Min(1.0, math.Max(0.0, (cpuUtil-0.80)*2.0)),
		}
	}
}

// evaluatePerformance tallies SLO violations and cluster-wide metrics.
func (cs *ClusterSimulator) evaluatePerformance(policy Policy, placed, rejected, lsCount int) *MetricsReport {
	sloViolations := 0
	totalRunqUs := 0.0
	maxRunqUs := 0.0
	totalCPUUtil := 0.0
	totalMemUtil := 0.0

	for _, n := range cs.nodes {
		cpuUtil := float64(n.AllocatedCPU) / float64(n.CPUCapacityMillis) * 100.0
		memUtil := float64(n.AllocatedMemory) / float64(n.MemoryCapacityBytes) * 100.0
		totalCPUUtil += cpuUtil
		totalMemUtil += memUtil

		runq := 0.0
		if n.LastTelemetry != nil {
			runq = n.LastTelemetry.RunqueueLatencyP99Us
		}
		totalRunqUs += runq
		if runq > maxRunqUs {
			maxRunqUs = runq
		}

		// Check each latency-sensitive workload on this node
		for _, w := range n.ActiveWorkloads {
			if w.Class == model.ClassLatencySensitive && w.SLO != nil {
				// Simulated p99 latency in ms: base response time + runqueue penalty
				simulatedP99Ms := 15.0 + (runq / 100.0) // 1000us runqueue adds 10ms p99 jitter
				if simulatedP99Ms > w.SLO.TargetP99Ms {
					sloViolations++
				}
			}
		}
	}

	nodeCount := float64(len(cs.nodes))
	violationRate := 0.0
	if lsCount > 0 {
		violationRate = (float64(sloViolations) / float64(lsCount)) * 100.0
	}

	return &MetricsReport{
		Policy:                policy,
		Seed:                  cs.cfg.Seed,
		TotalWorkloads:        len(cs.workloads),
		PlacedWorkloads:       placed,
		RejectedWorkloads:     rejected,
		LatencySensWorkloads:  lsCount,
		SLOViolations:         sloViolations,
		SLOViolationRate:      violationRate,
		AvgNodeCPUUtilization: totalCPUUtil / nodeCount,
		AvgNodeMemUtilization: totalMemUtil / nodeCount,
		AvgRunqueueLatencyUs:  totalRunqUs / nodeCount,
		MaxRunqueueLatencyUs:  maxRunqUs,
	}
}
