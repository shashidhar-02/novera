# ASTRA Technical Architecture Document

**Document ID:** ASTRA-ARCH-001  
**Status:** Approved  
**Version:** v0.1.0  

---

## 1. System Mission

ASTRA addresses the fundamental limitation of static resource reservation in distributed orchestration: **runtime microarchitectural and kernel queue contention**.

By continuously ingesting sub-second Linux Pressure Stall Information (PSI), cgroups v2 throttling metrics, and eBPF runqueue wait latency, ASTRA replaces static heuristics with a feedback-driven closed loop.

---

## 2. Mathematical Scoring Pipeline

Every candidate node $N$ that satisfies hard resource constraints is evaluated using a normalized composite objective function:

$$S(N, W) = w_{\text{cap}} \cdot S_{\text{cap}}(N, W) + w_{\text{press}} \cdot S_{\text{press}}(N) + w_{\text{interf}} \cdot S_{\text{interf}}(N, W)$$

Subject to:
$$w_{\text{cap}} + w_{\text{press}} + w_{\text{interf}} = 1.0, \quad \forall w \ge 0$$

- $S_{\text{cap}}$: Rewards balanced CPU and Memory utilization while penalizing lopsided resource consumption.
- $S_{\text{press}}$: Penalizes kernel CPU/Memory pressure stalls and runqueue latency:
  $$\text{Penalty} = \min(1.0, \frac{\text{PSI}_{\text{cpu}}}{20.0}) \times 40 + \min(1.0, \frac{\text{PSI}_{\text{mem}}}{15.0}) \times 30 + \min(1.0, \frac{\text{Runq}_{\text{p99}}}{10000.0}) \times 30$$
- $S_{\text{interf}}$: Evaluates empirical cross-class contention penalties (e.g. LatencySensitive vs ThroughputBatch).

---

## 3. Graceful Telemetry Degradation Invariant

If node telemetry is missing, stale ($> 5\text{s}$), or exhibits future clock skew:
1. $S_{\text{press}}$ transitions to a neutral default of $50.0$.
2. The score degrades gracefully to capacity and interference factors:
   $$S(N, W) \leftarrow \frac{w_{\text{cap}} \cdot S_{\text{cap}} + w_{\text{interf}} \cdot S_{\text{interf}}}{w_{\text{cap}} + w_{\text{interf}}}$$
3. The node is flagged as `TelemetryDegraded` in the explainable audit log.

