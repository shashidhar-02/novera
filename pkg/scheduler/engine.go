// Package scheduler implements the core ASTRA filtering, scoring, and placement engine.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"astra/pkg/interference"
	"astra/pkg/model"
)

var (
	ErrNoFitNodes        = errors.New("no candidate node satisfies hard placement constraints")
	ErrNilWorkload       = errors.New("workload must not be nil")
	ErrNoRegisteredNodes = errors.New("no nodes registered in cluster state")
)

// Config configures scheduler weights and safety thresholds.
type Config struct {
	WeightCapacity     float64       `json:"weight_capacity"`
	WeightPressure     float64       `json:"weight_pressure"`
	WeightInterference float64       `json:"weight_interference"`
	MaxTelemetryAge    time.Duration `json:"max_telemetry_age"`
}

// DefaultConfig returns balanced production defaults.
func DefaultConfig() Config {
	return Config{
		WeightCapacity:     0.30,
		WeightPressure:     0.40,
		WeightInterference: 0.30,
		MaxTelemetryAge:    5 * time.Second,
	}
}

// CandidateScore records detailed scoring components for a candidate node.
type CandidateScore struct {
	NodeID            string   `json:"node_id"`
	TotalScore        float64  `json:"total_score"`
	CapacityScore     float64  `json:"capacity_score"`
	PressureScore     float64  `json:"pressure_score"`
	InterferenceScore float64  `json:"interference_score"`
	TelemetryDegraded bool     `json:"telemetry_degraded"`
	Reasons           []string `json:"reasons"`
}

// PlacementDecision documents an explainable scheduling choice.
type PlacementDecision struct {
	WorkloadID        string            `json:"workload_id"`
	SelectedNodeID    string            `json:"selected_node_id"`
	SelectedScore     float64           `json:"selected_score"`
	CandidateScores   []CandidateScore  `json:"candidate_scores"`
	FilteredNodes     map[string]string `json:"filtered_nodes"` // NodeID -> rejection reason
	DecidedAt         time.Time         `json:"decided_at"`
	TelemetryDegraded bool              `json:"telemetry_degraded"`
	Explanation       string            `json:"explanation"`
}

// Engine performs candidate filtering and interference-aware scoring.
type Engine struct {
	cfg          Config
	interference interference.Model
}

// NewEngine constructs an ASTRA scheduling engine with validated, normalized weights.
func NewEngine(cfg Config, im interference.Model) *Engine {
	if im == nil {
		im = interference.NewDefaultMatrixModel()
	}
	// Sanitize weights
	if math.IsNaN(cfg.WeightCapacity) || cfg.WeightCapacity < 0 {
		cfg.WeightCapacity = 0.30
	}
	if math.IsNaN(cfg.WeightPressure) || cfg.WeightPressure < 0 {
		cfg.WeightPressure = 0.40
	}
	if math.IsNaN(cfg.WeightInterference) || cfg.WeightInterference < 0 {
		cfg.WeightInterference = 0.30
	}
	if cfg.MaxTelemetryAge <= 0 {
		cfg.MaxTelemetryAge = 5 * time.Second
	}

	sum := cfg.WeightCapacity + cfg.WeightPressure + cfg.WeightInterference
	if sum <= 0 {
		cfg = DefaultConfig()
	} else {
		cfg.WeightCapacity /= sum
		cfg.WeightPressure /= sum
		cfg.WeightInterference /= sum
	}

	return &Engine{
		cfg:          cfg,
		interference: im,
	}
}

// Schedule evaluates candidate nodes against the workload and selects the optimal node.
// Respects context cancellation and guarantees deterministic, explainable output.
func (e *Engine) Schedule(ctx context.Context, w *model.Workload, nodes map[string]*model.Node, now time.Time) (*PlacementDecision, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if w == nil {
		return nil, ErrNilWorkload
	}
	if err := w.Validate(); err != nil {
		return nil, fmt.Errorf("invalid workload: %w", err)
	}
	if len(nodes) == 0 {
		return nil, ErrNoRegisteredNodes
	}

	decision := &PlacementDecision{
		WorkloadID:      w.ID,
		FilteredNodes:   make(map[string]string),
		CandidateScores: make([]CandidateScore, 0, len(nodes)),
		DecidedAt:       now,
	}

	// 1. FILTER PHASE (Hard Constraints)
	var candidates []*model.Node
	for nodeID, n := range nodes {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if n == nil {
			decision.FilteredNodes[nodeID] = "node reference is nil"
			continue
		}
		if reason, ok := e.filterNode(w, n); !ok {
			decision.FilteredNodes[n.ID] = reason
		} else {
			candidates = append(candidates, n)
		}
	}

	if len(candidates) == 0 {
		return decision, ErrNoFitNodes
	}

	// 2. SCORE PHASE
	for _, n := range candidates {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		cs := e.scoreNode(w, n, now)
		decision.CandidateScores = append(decision.CandidateScores, cs)
	}

	// Sort candidates by total score descending; tie-break deterministically by NodeID ascending.
	// Explicitly guards against NaN in comparison.
	sort.Slice(decision.CandidateScores, func(i, j int) bool {
		sI := decision.CandidateScores[i].TotalScore
		sJ := decision.CandidateScores[j].TotalScore
		if math.IsNaN(sI) {
			return false
		}
		if math.IsNaN(sJ) {
			return true
		}
		if math.Abs(sI-sJ) > 0.0001 {
			return sI > sJ
		}
		return decision.CandidateScores[i].NodeID < decision.CandidateScores[j].NodeID
	})

	best := decision.CandidateScores[0]
	decision.SelectedNodeID = best.NodeID
	decision.SelectedScore = best.TotalScore
	decision.TelemetryDegraded = best.TelemetryDegraded
	decision.Explanation = fmt.Sprintf("Selected node %s with score %.2f (cap: %.2f, press: %.2f, interf: %.2f)",
		best.NodeID, best.TotalScore, best.CapacityScore, best.PressureScore, best.InterferenceScore)

	return decision, nil
}

// filterNode checks hard resource constraints.
func (e *Engine) filterNode(w *model.Workload, n *model.Node) (string, bool) {
	if n == nil {
		return "node reference is nil", false
	}
	if n.CPUCapacityMillis <= 0 {
		return fmt.Sprintf("node %s has invalid CPU capacity (%d m)", n.ID, n.CPUCapacityMillis), false
	}
	if n.MemoryCapacityBytes <= 0 {
		return fmt.Sprintf("node %s has invalid memory capacity (%d bytes)", n.ID, n.MemoryCapacityBytes), false
	}

	availCPU := n.AvailableCPU()
	if availCPU < w.CPURequestMillis {
		return fmt.Sprintf("insufficient CPU: requested %d m, available %d m", w.CPURequestMillis, availCPU), false
	}

	availMem := n.AvailableMemory()
	if availMem < w.MemoryRequestBytes {
		return fmt.Sprintf("insufficient memory: requested %d bytes, available %d bytes", w.MemoryRequestBytes, availMem), false
	}
	return "", true
}

// scoreNode computes capacity, pressure, and interference metrics with zero-division protection.
func (e *Engine) scoreNode(w *model.Workload, n *model.Node, now time.Time) CandidateScore {
	var reasons []string

	// 1. Capacity Score [0 - 100]
	capScore := 0.0
	if n.CPUCapacityMillis > 0 && n.MemoryCapacityBytes > 0 {
		cpuUtil := float64(n.AllocatedCPU+w.CPURequestMillis) / float64(n.CPUCapacityMillis)
		memUtil := float64(n.AllocatedMemory+w.MemoryRequestBytes) / float64(n.MemoryCapacityBytes)
		avgUtil := (cpuUtil + memUtil) / 2.0
		diffUtil := math.Abs(cpuUtil - memUtil)
		capScore = math.Max(0.0, math.Min(100.0, 100.0*(1.0-avgUtil-diffUtil*0.5)))
	}

	// 2. Pressure Score [0 - 100]
	pressureScore := 100.0
	telemetryDegraded := false

	if n.LastTelemetry == nil || n.LastTelemetry.IsStale(now, e.cfg.MaxTelemetryAge) {
		// Telemetry missing, stale, or future clock skew: degrade gracefully
		telemetryDegraded = true
		pressureScore = 50.0 // Conservative neutral default
		reasons = append(reasons, "telemetry stale, missing, or clock-skewed: pressure score set to neutral default (50.0)")
	} else {
		// Validate and bound raw telemetry inputs to prevent NaN or extreme values
		cpuStall := math.Max(0.0, math.Min(100.0, n.LastTelemetry.CPUPressure.SomeAvg10))
		memStall := math.Max(0.0, math.Min(100.0, n.LastTelemetry.MemoryPressure.SomeAvg10))
		runqUs := math.Max(0.0, n.LastTelemetry.RunqueueLatencyP99Us)

		cpuPenalty := math.Min(1.0, cpuStall/20.0) * 40.0
		memPenalty := math.Min(1.0, memStall/15.0) * 30.0
		runqPenalty := math.Min(1.0, runqUs/10000.0) * 30.0

		totalPenalty := cpuPenalty + memPenalty + runqPenalty
		pressureScore = math.Max(0.0, math.Min(100.0, 100.0-totalPenalty))

		if cpuPenalty > 15.0 {
			reasons = append(reasons, fmt.Sprintf("elevated CPU pressure: PSI some_avg10=%.2f", cpuStall))
		}
		if memPenalty > 10.0 {
			reasons = append(reasons, fmt.Sprintf("elevated memory pressure: PSI some_avg10=%.2f", memStall))
		}
		if runqPenalty > 10.0 {
			reasons = append(reasons, fmt.Sprintf("elevated runqueue latency: p99=%.1fus", runqUs))
		}
	}

	// 3. Interference Score [0 - 100]
	interfPenalty, interfReasons := e.interference.Evaluate(w, n)
	interfScore := math.Max(0.0, math.Min(100.0, 100.0-interfPenalty))
	reasons = append(reasons, interfReasons...)

	// Weighted Total Score
	var total float64
	if telemetryDegraded {
		// Fallback to capacity + interference only
		subSum := e.cfg.WeightCapacity + e.cfg.WeightInterference
		if subSum > 0 {
			total = (e.cfg.WeightCapacity*capScore + e.cfg.WeightInterference*interfScore) / subSum
		} else {
			total = capScore
		}
	} else {
		total = (e.cfg.WeightCapacity * capScore) +
			(e.cfg.WeightPressure * pressureScore) +
			(e.cfg.WeightInterference * interfScore)
	}

	if math.IsNaN(total) {
		total = 0.0
	}

	return CandidateScore{
		NodeID:            n.ID,
		TotalScore:        math.Max(0.0, math.Min(100.0, total)),
		CapacityScore:     capScore,
		PressureScore:     pressureScore,
		InterferenceScore: interfScore,
		TelemetryDegraded: telemetryDegraded,
		Reasons:           reasons,
	}
}
