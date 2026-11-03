# RFC-0002: Workload Interference Model Specification

**RFC Number:** 0002  
**Title:** Empirical Cross-Class Workload Interference & Contention Modeling  
**Author:** ASTRA Architecture Team  
**Status:** Approved / Implemented in v0.1.0  
**Target Subsystem:** `pkg/interference`  

---

## 1. Problem & Motivation

Static resource allocation assumes that independent workloads co-located on a physical CPU socket do not interfere with each other if their CPU and Memory requests fit within allocatable capacity. 

In production systems, shared microarchitectural and OS primitives (L3 cache lines, memory bus bandwidth, CPU runqueues, and I/O elevator queues) become contention points. Specifically:
- **Latency-Sensitive (LS) + Throughput-Batch (TH):** High thread churn and long CFS timeslices from batch workloads induce context-switch delays and queue tail-latency spikes ($p99$ degradation $> 300\%$).
- **Latency-Sensitive (LS) + I/O-Intensive (IO):** Heavy page-cache dirtying and writeback blocking stall latency-sensitive memory allocations.

ASTRA introduces an empirical cross-class contention model to steer incoming workloads away from hostile co-locations.

---

## 2. Theoretical Classification

ASTRA explicitly separates:
1. **Raw Observation:** Kernel telemetry (PSI, runqueue latency, CFS throttled ratios).
2. **Correlation:** Observed co-occurrence of workload classes with elevated PSI.
3. **Hypothesis:** Specific class pairs (LS + TH) generate microarchitectural contention.
4. **Empirical Policy:** Penalizing high-risk class co-locations during node scoring.

ASTRA makes **no claims of unverified causal discovery**; the interference model operates on empirically validated affinity matrices.

---

## 3. Mathematical Specification

Given an incoming workload $W_{\text{in}}$ of class $C(W_{\text{in}})$ and a candidate node $N$ with active workloads $\mathcal{A}(N)$:

$$\text{Penalty}(W_{\text{in}}, N) = \min\left(100.0, \sum_{C \in \text{Classes}} \min\left(60.0, \mathcal{M}[C(W_{\text{in}})][C] \times \text{Count}(C, N)\right)\right)$$

Where $\mathcal{M}$ is the empirical penalty matrix:

| Incoming \ Active | LatencySensitive | ThroughputBatch | IOIntensive |
|---|---|---|---|
| **LatencySensitive** | 5.0 (L3 share) | 35.0 (Runqueue stall) | 25.0 (I/O stall) |
| **ThroughputBatch** | 5.0 | 0.0 (Fair share) | 15.0 (Mem bandwidth) |
| **IOIntensive** | 10.0 | 15.0 | 30.0 (Queue head-of-line) |

---

## 4. Security & Failure Invariants

- **Nil & Boundary Safety:** Evaluating `nil` workloads, `nil` nodes, or unclassified tasks yields `0.0` penalty with a diagnostic reason, preventing scheduler crashes.
- **Explainability:** For every evaluated node, the model emits explicit human-readable reasons (e.g. `"high contention risk: co-locating LatencySensitive with ThroughputBatch"`).

