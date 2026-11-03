# RFC-0001: ASTRA System Architecture

**RFC Number:** 0001  
**Title:** ASTRA Distributed Telemetry & Causal Workload Scheduling Architecture  
**Author:** ASTRA Architecture Team  
**Status:** Proposed / Under Review  
**Created:** 2026-11-03  
**Target Version:** v0.1.0  

---

## 1. Problem Statement

Modern cloud orchestration frameworks (e.g., Kubernetes `kube-scheduler`, Nomad) place workloads based primarily on declared static constraints:
$$\text{Allocatable}(N) = \text{Capacity}(N) - \sum_{W \in \text{Assigned}(N)} \text{Requests}(W)$$

This abstraction assumes that declared requests accurately reflect runtime behavior and that workloads sharing a physical node are performance-isolated. In reality:
1. **Dynamic Interference:** Co-located workloads contend for unmanaged hardware resources: L3 cache lines, memory bus bandwidth, CPU runqueue capacity, I/O queues, and socket-level interconnects.
2. **Hidden Latency Stalls:** Latency-sensitive workloads experience tail latency degradation ($p99$ spikes of 300%–1000%) caused by CPU runqueue starvation (`sched_switch` wait delays) and cgroup CFS quota throttling, despite node CPU utilization remaining below 60%.
3. **No Closed-Loop Feedback:** Standard schedulers operate as open-loop systems: once a workload is bound to a node, runtime performance degradation does not inform future placement decisions unless an out-of-memory (OOM) or eviction threshold is crossed.

---

## 2. Goals & Non-Goals

### 2.1 Goals
- **Closed-Loop Kernel Telemetry:** Ingest Linux cgroups v2, Pressure Stall Information (PSI), and eBPF runqueue metrics into a centralized cluster state with sub-second temporal resolution.
- **Interference-Aware Scoring:** Evaluate candidate nodes using a composite score incorporating static capacity, measured subsystem pressure (CPU, memory, I/O), and workload co-location compatibility.
- **SLO-Centric Objective Function:** Prioritize nodes that minimize the predicted probability of $p99$ tail latency violations for latency-sensitive (LS) workloads.
- **Explainable Decisions:** Every placement decision must emit a deterministic audit record detailing candidate filtering reasons, scoring breakdown, dominant pressure factors, and confidence level.
- **Safe Fallback Invariants:** When telemetry is stale ($> 5\text{s}$), missing, or corrupted, the scheduler must gracefully fall back to deterministic capacity-based placement without stalling or crashing.
- **Modular Portability:** Support three operational modes sharing identical core interfaces:
  1. *Simulator Mode:* Pure user-space deterministic simulation for testing and benchmarking on any OS (including development machines).
  2. *Linux Standalone Mode:* Direct node placement via Linux cgroups and node-agent telemetry.
  3. *Kubernetes Mode:* Plugin integration via the Kubernetes Scheduling Framework (`Score` / `Filter`).

### 2.2 Non-Goals
- **Replacing the Linux Kernel Scheduler:** ASTRA is an orchestrator/cluster scheduler, not an in-kernel task scheduler (like CFS or EEVDF).
- **Replacing Kubernetes:** ASTRA integrates with the Kubernetes control plane rather than reimplementing pods, services, or controllers.
- **Unbounded Machine Learning in the Critical Path:** No non-deterministic LLM or deep neural network will be placed in the synchronous scheduling loop. Any heuristic or regression model must execute in $< 5\text{ms}$.
- **Autonomous Destructive Production Remediation:** Initial versions will not autonomously kill or evict active workloads; migrations are subject to configurable disruption budgets and policy engine authorization.

---

## 3. High-Level Architecture & Component Boundaries

```text
                                +-------------------------------------+
                                |      Workload Submission API        |
                                |  (Kubernetes Pod / Standalone Job)  |
                                +------------------+------------------+
                                                   |
                                                   v
+---------------------------------------------------------------------------------------------------+
|                                      ASTRA CONTROL PLANE                                          |
|                                                                                                   |
|  +---------------------+      +------------------------+      +--------------------------------+  |
|  |   Filter Engine     | ---> |    Scoring Pipeline    | ---> |    Placement & Explain Engine  |  |
|  | (Hard Constraints)  |      |  - Static Capacity     |      |  - Node Selection              |  |
|  | - CPU/Mem Limits    |      |  - Node PSI Pressure   |      |  - Audit Trace Emission        |  |
|  | - Taints/Tolerations|      |  - Interference Matrix |      |  - Binding Action              |  |
|  | - Hard Anti-Affinity|      |  - SLO Latency Risk    |      +---------------+----------------+  |
|  +---------------------+      +-----------^------------+                      |                   |
|                                           |                                   |                   |
|                               +-----------+------------+                      |                   |
|                               |   Cluster State Store  |                      |                   |
|                               | (Nodes, Workloads, PSI)|                      |                   |
|                               +-----------^------------+                      |                   |
+-------------------------------------------|-----------------------------------|-------------------+
                                            | Telemetry                         | Placement
                                            | Stream (gRPC/mTLS)                | Binding
                                            |                                   |
         +----------------------------------+------------------+                |
         |                                                     |                |
         v                                                     v                v
+----------------------------------+         +----------------------------------+
|         ASTRA NODE AGENT         |         |         ASTRA NODE AGENT         |
|  (Node A - Linux Kernel 5.8+)    |         |  (Node B - Linux Kernel 5.8+)    |
|                                  |         |                                  |
|  +----------------------------+  |         |  +----------------------------+  |
|  |   cgroups v2 Telemetry     |  |         |  |   cgroups v2 Telemetry     |  |
|  |  - cpu.stat (throttling)   |  |         |  |  - cpu.stat (throttling)   |  |
|  |  - memory.current / high   |  |         |  |  - memory.current / high   |  |
|  +----------------------------+  |         |  +----------------------------+  |
|  +----------------------------+  |         |  +----------------------------+  |
|  |   Linux PSI Collector      |  |         |  |   Linux PSI Collector      |  |
|  |  - /proc/pressure/{cpu,mem}|  |         |  |  - /proc/pressure/{cpu,mem}|  |
|  +----------------------------+  |         |  +----------------------------+  |
|  +----------------------------+  |         |  +----------------------------+  |
|  |   eBPF Kernel Probes       |  |         |  |   eBPF Kernel Probes       |  |
|  |  - sched:sched_switch      |  |         |  |  - sched:sched_switch      |  |
|  |  - runqueue latency hist   |  |         |  |  - runqueue latency hist   |  |
|  +----------------------------+  |         |  +----------------------------+  |
+----------------------------------+         +----------------------------------+
```

---

## 4. Core Data Structures & Interfaces

The interfaces are specified in standard Go to guarantee cross-platform compilation, zero-overhead interfaces, and deterministic unit testing.

### 4.1 Workload Model

```go
package model

import "time"

// WorkloadClass designates the performance profile and priority tier.
type WorkloadClass string

const (
    ClassLatencySensitive WorkloadClass = "LatencySensitive" // Strict p99 SLO (e.g., gRPC, Search, Redis)
    ClassThroughputBatch  WorkloadClass = "ThroughputBatch"  // CPU/Memory bound, latency-tolerant (e.g., ETL, ML)
    ClassIOIntensive      WorkloadClass = "IOIntensive"      // High disk/network IO (e.g., DB compaction)
)

// SLOSpec defines explicit latency and availability thresholds.
type SLOSpec struct {
    TargetP99Ms float64       `json:"target_p99_ms"`
    MaxAllowedJitterMs float64 `json:"max_allowed_jitter_ms"`
}

// Workload represents an schedulable entity (equivalent to a Pod or Task).
type Workload struct {
    ID          string            `json:"id"`
    Namespace   string            `json:"namespace"`
    Class       WorkloadClass     `json:"class"`
    CPURequestMillis int64        `json:"cpu_request_millis"` // 1000 = 1 vCPU
    MemoryRequestBytes int64      `json:"memory_request_bytes"`
    SLO         *SLOSpec          `json:"slo,omitempty"`
    Labels      map[string]string `json:"labels,omitempty"`
}
```

### 4.2 Node & Telemetry Model

```go
package model

import "time"

// PSIPressure represents the Linux Pressure Stall Information for a resource.
type PSIPressure struct {
    SomeAvg10  float64 `json:"some_avg10"`  // % stalled in last 10s
    SomeAvg60  float64 `json:"some_avg60"`  // % stalled in last 60s
    FullAvg10  float64 `json:"full_avg10"`  // % all tasks stalled in last 10s
    FullAvg60  float64 `json:"full_avg60"`  // % all tasks stalled in last 60s
}

// NodeTelemetry represents sub-second measurements collected by the Node Agent.
type NodeTelemetry struct {
    NodeID              string      `json:"node_id"`
    CollectedAt         time.Time   `json:"collected_at"`
    SequenceNumber      uint64      `json:"sequence_number"`
    CPUPressure         PSIPressure `json:"cpu_pressure"`
    MemoryPressure      PSIPressure `json:"memory_pressure"`
    IOPressure          PSIPressure `json:"io_pressure"`
    RunqueueLatencyP99Us float64     `json:"runqueue_latency_p99_us"` // Microseconds
    CFSThrottledRatio   float64     `json:"cfs_throttled_ratio"`     // throttled_time / total_time
}

// Node represents physical or virtual host capacity and current state.
type Node struct {
    ID                 string          `json:"id"`
    CPUCapacityMillis  int64           `json:"cpu_capacity_millis"`
    MemoryCapacityBytes int64          `json:"memory_capacity_bytes"`
    AllocatedCPU       int64           `json:"allocated_cpu"`
    AllocatedMemory    int64           `json:"allocated_memory"`
    ActiveWorkloads    []string        `json:"active_workloads"`
    LastTelemetry      *NodeTelemetry  `json:"last_telemetry,omitempty"`
}
```

### 4.3 Scheduler Interface

```go
package scheduler

import (
    "context"
    "astra/pkg/model"
)

// CandidateScore records the scoring evaluation for a single node.
type CandidateScore struct {
    NodeID          string             `json:"node_id"`
    TotalScore      float64            `json:"total_score"`      // Range [0.0, 100.0]
    CapacityScore   float64            `json:"capacity_score"`   // Available resource ratio
    PressureScore   float64            `json:"pressure_score"`   // Inverse PSI penalty
    InterferenceScore float64          `json:"interference_score"` // Co-location penalty
    Reasons         []string           `json:"reasons"`
}

// PlacementDecision provides explainable, reproducible records of placement.
type PlacementDecision struct {
    WorkloadID     string            `json:"workload_id"`
    SelectedNodeID string            `json:"selected_node_id"`
    SelectedScore  float64           `json:"selected_score"`
    Scores         []CandidateScore  `json:"candidate_scores"`
    FilteredNodes  map[string]string `json:"filtered_nodes"` // NodeID -> rejection reason
    DecidedAt      int64             `json:"decided_at_unix_nano"`
    Confidence     float64           `json:"confidence"`
}

// Engine defines the scheduling interface.
type Engine interface {
    Schedule(ctx context.Context, w *model.Workload) (*PlacementDecision, error)
}
```

---

## 5. Scoring Algorithm & Objective Formulation

The candidate scoring pipeline evaluates all non-filtered nodes $N \in \text{Candidates}$ using a normalized multi-objective function:

$$S(N, W) = w_{\text{cap}} \cdot S_{\text{cap}}(N, W) + w_{\text{press}} \cdot S_{\text{press}}(N) + w_{\text{interf}} \cdot S_{\text{interf}}(N, W)$$

Subject to:
$$w_{\text{cap}} + w_{\text{press}} + w_{\text{interf}} = 1.0, \quad \forall w \ge 0$$

### 5.1 Capacity Score ($S_{\text{cap}}$)
Rewards balanced resource utilization across CPU and Memory:
$$U_{\text{cpu}}(N, W) = \frac{\text{AllocatedCPU}(N) + \text{RequestCPU}(W)}{\text{CapacityCPU}(N)}$$
$$U_{\text{mem}}(N, W) = \frac{\text{AllocatedMem}(N) + \text{RequestMem}(W)}{\text{CapacityMem}(N)}$$
$$S_{\text{cap}}(N, W) = 100 \times \left(1.0 - \frac{U_{\text{cpu}} + U_{\text{mem}}}{2} - |U_{\text{cpu}} - U_{\text{mem}}|\right)$$

### 5.2 Pressure Score ($S_{\text{press}}$)
Penalizes nodes with elevated kernel PSI stalls or runqueue delays:
$$\text{Penalty}_{\text{cpu}} = \min(1.0, \frac{\text{PSI}_{\text{cpu:some\_10}}}{20.0}) \times 40$$
$$\text{Penalty}_{\text{mem}} = \min(1.0, \frac{\text{PSI}_{\text{mem:some\_10}}}{15.0}) \times 30$$
$$\text{Penalty}_{\text{runq}} = \min(1.0, \frac{\text{RunqueueLatencyP99Us}}{10000.0}) \times 30$$
$$S_{\text{press}}(N) = \max\left(0.0, 100.0 - (\text{Penalty}_{\text{cpu}} + \text{Penalty}_{\text{mem}} + \text{Penalty}_{\text{runq}})\right)$$

### 5.3 Interference Score ($S_{\text{interf}}$)
Measures the historical degradation penalty of co-scheduling workload class $C(W)$ on node $N$ alongside currently running classes:
$$S_{\text{interf}}(N, W) = 100.0 - \sum_{W' \in \text{Active}(N)} \mathcal{M}_{\text{interf}}(C(W), C(W'))$$
Where $\mathcal{M}_{\text{interf}}$ is the symmetric interference penalty matrix:
- $\text{LS} \times \text{LS}$: Low penalty ($5.0$)
- $\text{LS} \times \text{Batch}$: High penalty ($35.0$) — prevents batch CPU thrashing latency-sensitive p99
- $\text{LS} \times \text{IO}$: High penalty ($30.0$) — prevents IO queue starvation
- $\text{Batch} \times \text{Batch}$: Zero penalty ($0.0$) — batch jobs can share without SLO penalties

---

## 6. Failure Modes & Safety Invariants

| Failure Scenario | Detection Mechanism | Immediate Fallback Action | Recovery Strategy |
|---|---|---|---|
| **Stale Node Telemetry** | Timestamp delta $\Delta t > 5.0\text{s}$ | Fallback to Static Capacity Score ($w_{\text{press}} \to 0$); flag node as `TelemetryDegraded`. | Restore normal weights once continuous valid heartbeats resume. |
| **Agent Disconnection / Partition** | Missed $3 \times$ keepalive cycles | Mark node `Unreachable`; do not assign new workloads. | Re-sync full node state upon reconnect. |
| **Malformed / Out-of-Bounds Metrics** | Schema validation bounds ($[0, 100]\%$) | Drop sample, increment telemetry error counter; use prior exponential moving average. | Alert operator if error rate exceeds 5%. |
| **Capacity Saturated Cluster** | All candidate nodes filtered | Return deterministic `ErrNoFitNodes` with exhaustive rejection reasons. | Workload enqueued in prioritized pending queue. |

---

## 7. Security Architecture

1. **Least Privilege Node Agent:**
   - eBPF telemetry runs under `CAP_BPF` and `CAP_PERFMON` on Linux 5.8+ without granting full `CAP_SYS_ADMIN`.
   - Read-only access to `/proc/pressure` and `/sys/fs/cgroup`.
   - Node agent cannot execute shell commands, terminate arbitrary host processes, or alter host network interfaces.
2. **Control Plane Isolation:**
   - Communication between Node Agents and the Control Plane is authenticated and encrypted via mutual TLS (mTLS).
   - In Kubernetes mode, service accounts are bound to minimal RBAC permissions (Pod binding, node watch, lease update).
3. **Auditability:**
   - Every placement decision is cryptographically logged with its exact inputs, scores, and timestamp to prevent untraceable cluster shifts.

---

## 8. Verification & Benchmarking Strategy

### 8.1 Verification (Testing Pyramid)
1. **Unit Tests:**
   - 100% test coverage on candidate filtering logic, mathematical scoring formulas, and edge conditions (zero capacity, negative pressure, overflow).
2. **Deterministic Simulation Tests:**
   - Property-based testing using fixed PRNG seeds to verify that workloads placed by ASTRA consistently exhibit lower simulated pressure collisions than a random or naive round-robin scheduler.
3. **Fault-Injection Tests:**
   - Inject synthetic telemetry dropouts, clock skews, and corrupted payload frames to prove safety invariants hold.

### 8.2 Benchmarking Protocol
- **Latency Target:** Complete placement decision for 1,000 nodes in $< 10\text{ms}$.
- **Throughput Target:** $> 500$ scheduling decisions per second on a single control-plane core.
- **Overhead Budget:** Node agent CPU usage $< 1.0\%$ of a single core; memory footprint $< 32\text{MB}$.
