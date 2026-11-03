# RFC-0003: Core Scheduling & Placement Engine

**RFC Number:** 0003  
**Title:** Multi-Objective Scoring, Hard Constraints, and Telemetry Degradation Invariants  
**Author:** ASTRA Architecture Team  
**Status:** Approved / Implemented in v0.1.0  
**Target Subsystem:** `pkg/scheduler`  

---

## 1. Problem & Motivation

Standard cluster schedulers rely on heuristic packing (e.g. `NodeResourcesBalancedAllocation`) that only optimizes static reservations. When a node experiences kernel pressure stalls (PSI) or telemetry loss, the scheduler is blind to node health degradation.

ASTRA's scheduler engine provides a mathematically bounded, multi-objective scoring pipeline with deterministic tie-breaking and graceful telemetry degradation.

---

## 2. Specification

### 2.1 Filter Phase (Hard Constraints)
Every candidate node must strictly satisfy:
$$\text{AvailableCPU}(N) \ge \text{RequestCPU}(W)$$
$$\text{AvailableMemory}(N) \ge \text{RequestMemory}(W)$$
$$\text{CapacityCPU}(N) > 0, \quad \text{CapacityMemory}(N) > 0$$

If any condition fails, the node is eliminated, and the exact failure reason is recorded in `PlacementDecision.FilteredNodes`.

### 2.2 Score Phase (Normalized Multi-Objective)
For all filtered candidates, composite scoring evaluates:
$$S(N, W) = w_{\text{cap}} \cdot S_{\text{cap}} + w_{\text{press}} \cdot S_{\text{press}} + w_{\text{interf}} \cdot S_{\text{interf}}$$

- $S_{\text{cap}}$: Rewards balanced resource utilization across CPU and Memory:
  $$S_{\text{cap}} = 100 \times \left(1.0 - \frac{U_{\text{cpu}} + U_{\text{mem}}}{2} - |U_{\text{cpu}} - U_{\text{mem}}| \times 0.5\right)$$
- $S_{\text{press}}$: Penalizes kernel PSI stalls and runqueue delays:
  $$\text{Penalty} = \min(1.0, \frac{\text{PSI}_{\text{cpu}}}{20.0}) \times 40 + \min(1.0, \frac{\text{PSI}_{\text{mem}}}{15.0}) \times 30 + \min(1.0, \frac{\text{Runq}_{\text{p99}}}{10000.0}) \times 30$$
  $$S_{\text{press}} = \max(0.0, 100.0 - \text{Penalty})$$
- $S_{\text{interf}}$: Evaluates cross-class interference penalty from RFC-0002.

### 2.3 Stale Telemetry & Clock Skew Degradation
If telemetry is missing, uninitialized, older than `MaxTelemetryAge` ($5\text{s}$), or exhibits future clock skew:
$$\text{TelemetryDegraded} \leftarrow \text{true}$$
$$S_{\text{press}} \leftarrow 50.0 \quad (\text{Conservative Neutral Default})$$
$$S(N, W) \leftarrow \frac{w_{\text{cap}} \cdot S_{\text{cap}} + w_{\text{interf}} \cdot S_{\text{interf}}}{w_{\text{cap}} + w_{\text{interf}}}$$

---

## 3. Concurrency & Determinism Guarantees

- **Context Cancellation:** Synchronously honors `ctx.Err()` at every phase.
- **Deterministic Tie-Breaking:** When candidate total scores differ by $\le 0.0001$, nodes are ordered alphabetically by `NodeID`.
- **Thread Safety:** Pure re-entrant execution; no mutable state shared across concurrent `Schedule` invocations.

