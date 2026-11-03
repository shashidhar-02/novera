// Package model defines core entities and telemetry schemas for ASTRA.
package model

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// WorkloadClass designates the performance profile and isolation tier.
type WorkloadClass string

const (
	// ClassLatencySensitive denotes services with strict p99 latency SLOs (e.g., Redis, gRPC microservices, Search).
	ClassLatencySensitive WorkloadClass = "LatencySensitive"

	// ClassThroughputBatch denotes compute-heavy or memory-heavy jobs tolerant of queuing delays (e.g., ML training, ETL).
	ClassThroughputBatch WorkloadClass = "ThroughputBatch"

	// ClassIOIntensive denotes storage or network bandwidth intensive workloads (e.g., database compaction, backup).
	ClassIOIntensive WorkloadClass = "IOIntensive"
)

// SLOSpec defines explicit target bounds for a workload.
type SLOSpec struct {
	TargetP99Ms        float64 `json:"target_p99_ms"`
	MaxAllowedJitterMs float64 `json:"max_allowed_jitter_ms"`
}

// Workload represents an allocatable task or container group.
type Workload struct {
	ID                 string            `json:"id"`
	Namespace          string            `json:"namespace"`
	Class              WorkloadClass     `json:"class"`
	CPURequestMillis   int64             `json:"cpu_request_millis"` // 1000 = 1 core
	MemoryRequestBytes int64             `json:"memory_request_bytes"`
	SLO                *SLOSpec          `json:"slo,omitempty"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// Validate checks whether the workload specification is well-formed.
func (w *Workload) Validate() error {
	if w == nil {
		return errors.New("workload must not be nil")
	}
	if w.ID == "" {
		return errors.New("workload ID must not be empty")
	}
	if w.CPURequestMillis <= 0 {
		return fmt.Errorf("invalid CPURequestMillis %d: must be positive", w.CPURequestMillis)
	}
	if w.MemoryRequestBytes <= 0 {
		return fmt.Errorf("invalid MemoryRequestBytes %d: must be positive", w.MemoryRequestBytes)
	}
	switch w.Class {
	case ClassLatencySensitive, ClassThroughputBatch, ClassIOIntensive:
	default:
		return fmt.Errorf("unknown workload class: %s", w.Class)
	}

	if w.SLO != nil {
		if math.IsNaN(w.SLO.TargetP99Ms) || math.IsInf(w.SLO.TargetP99Ms, 0) || w.SLO.TargetP99Ms <= 0 {
			return fmt.Errorf("invalid SLO TargetP99Ms %.2f: must be a positive finite number", w.SLO.TargetP99Ms)
		}
		if math.IsNaN(w.SLO.MaxAllowedJitterMs) || math.IsInf(w.SLO.MaxAllowedJitterMs, 0) || w.SLO.MaxAllowedJitterMs < 0 {
			return fmt.Errorf("invalid SLO MaxAllowedJitterMs %.2f: must be a non-negative finite number", w.SLO.MaxAllowedJitterMs)
		}
	}
	return nil
}

// PSIPressure represents Linux Pressure Stall Information (PSI).
type PSIPressure struct {
	SomeAvg10 float64 `json:"some_avg10"` // Percentage [0.0 - 100.0] of time at least 1 task was stalled
	SomeAvg60 float64 `json:"some_avg60"`
	FullAvg10 float64 `json:"full_avg10"` // Percentage [0.0 - 100.0] of time all tasks were stalled
	FullAvg60 float64 `json:"full_avg60"`
}

// NodeTelemetry captures runtime metrics collected by the node agent from kernel / cgroups.
type NodeTelemetry struct {
	NodeID               string      `json:"node_id"`
	CollectedAt          time.Time   `json:"collected_at"`
	SequenceNumber       uint64      `json:"sequence_number"`
	CPUPressure          PSIPressure `json:"cpu_pressure"`
	MemoryPressure       PSIPressure `json:"memory_pressure"`
	IOPressure           PSIPressure `json:"io_pressure"`
	RunqueueLatencyP99Us float64     `json:"runqueue_latency_p99_us"` // Microseconds
	CFSThrottledRatio    float64     `json:"cfs_throttled_ratio"`     // [0.0 - 1.0]
}

// IsStale returns true if telemetry is older than the maxAllowedDuration,
// uninitialized, or exhibits unacceptable clock skew (future timestamp).
func (t *NodeTelemetry) IsStale(now time.Time, maxAllowedDuration time.Duration) bool {
	if t == nil || t.CollectedAt.IsZero() {
		return true
	}
	// Past staleness
	if now.Sub(t.CollectedAt) > maxAllowedDuration {
		return true
	}
	// Future timestamp clock skew protection
	if t.CollectedAt.Sub(now) > maxAllowedDuration {
		return true
	}
	return false
}

// Node represents physical or virtual host capacity and current state.
type Node struct {
	mu                  sync.RWMutex
	ID                  string               `json:"id"`
	CPUCapacityMillis   int64                `json:"cpu_capacity_millis"`
	MemoryCapacityBytes int64                `json:"memory_capacity_bytes"`
	AllocatedCPU        int64                `json:"allocated_cpu"`
	AllocatedMemory     int64                `json:"allocated_memory"`
	ActiveWorkloads     map[string]*Workload `json:"active_workloads"`
	LastTelemetry       *NodeTelemetry       `json:"last_telemetry,omitempty"`
	Labels              map[string]string    `json:"labels,omitempty"`
}

// NewNode initializes a clean Node instance.
func NewNode(id string, cpuMillis int64, memoryBytes int64) *Node {
	return &Node{
		ID:                  id,
		CPUCapacityMillis:   cpuMillis,
		MemoryCapacityBytes: memoryBytes,
		ActiveWorkloads:     make(map[string]*Workload),
		Labels:              make(map[string]string),
	}
}

// AvailableCPU returns unallocated CPU in millicores. Thread-safe.
func (n *Node) AvailableCPU() int64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	avail := n.CPUCapacityMillis - n.AllocatedCPU
	if avail < 0 {
		return 0
	}
	return avail
}

// AvailableMemory returns unallocated memory in bytes. Thread-safe.
func (n *Node) AvailableMemory() int64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	avail := n.MemoryCapacityBytes - n.AllocatedMemory
	if avail < 0 {
		return 0
	}
	return avail
}

// AssignWorkload binds a workload to this node, updating allocated resources. Thread-safe.
func (n *Node) AssignWorkload(w *Workload) error {
	if n == nil {
		return errors.New("node cannot be nil")
	}
	if w == nil {
		return errors.New("workload cannot be nil")
	}
	if err := w.Validate(); err != nil {
		return fmt.Errorf("invalid workload: %w", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Guard against duplicate assignment resource leak
	if _, exists := n.ActiveWorkloads[w.ID]; exists {
		return fmt.Errorf("workload %s is already assigned to node %s", w.ID, n.ID)
	}

	availCPU := n.CPUCapacityMillis - n.AllocatedCPU
	if availCPU < w.CPURequestMillis {
		return fmt.Errorf("insufficient CPU on node %s: requested %d m, available %d m", n.ID, w.CPURequestMillis, availCPU)
	}

	availMem := n.MemoryCapacityBytes - n.AllocatedMemory
	if availMem < w.MemoryRequestBytes {
		return fmt.Errorf("insufficient memory on node %s: requested %d bytes, available %d bytes", n.ID, w.MemoryRequestBytes, availMem)
	}

	n.AllocatedCPU += w.CPURequestMillis
	n.AllocatedMemory += w.MemoryRequestBytes
	n.ActiveWorkloads[w.ID] = w
	return nil
}

// RemoveWorkload unbinds a workload from this node. Thread-safe.
func (n *Node) RemoveWorkload(workloadID string) {
	if n == nil || workloadID == "" {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()

	if w, ok := n.ActiveWorkloads[workloadID]; ok {
		n.AllocatedCPU -= w.CPURequestMillis
		n.AllocatedMemory -= w.MemoryRequestBytes
		if n.AllocatedCPU < 0 {
			n.AllocatedCPU = 0
		}
		if n.AllocatedMemory < 0 {
			n.AllocatedMemory = 0
		}
		delete(n.ActiveWorkloads, workloadID)
	}
}

// GetActiveWorkloads returns a snapshot copy of active workloads for safe concurrent reads.
func (n *Node) GetActiveWorkloads() []*Workload {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()

	workloads := make([]*Workload, 0, len(n.ActiveWorkloads))
	for _, w := range n.ActiveWorkloads {
		if w != nil {
			workloads = append(workloads, w)
		}
	}
	return workloads
}
