# ASTRA Benchmark & Evaluation Plan

**Document ID:** ASTRA-PLAN-BENCH-001  
**Status:** Approved  

---

## 1. Evaluation Goals

1. Quantify the reduction in tail-latency ($p99$) SLO violations achieved by ASTRA compared to baseline static capacity scheduling under controlled noisy-neighbor contention.
2. Measure scheduler decision latency ($p50$, $p95$, $p99$) and maximum throughput (decisions/sec).
3. Validate that telemetry ingestion overhead in the node agent remains $< 1.0\%$ of a single CPU core.

---

## 2. Workload Synthesis Scenarios

| Profile | CPU Range | Memory Range | SLO Target | Contention Signature |
|---|---|---|---|---|
| **Latency-Sensitive (LS)** | $500-1000\text{m}$ | $512-1024\text{MB}$ | $p99 < 25-40\text{ms}$ | High context-switch sensitivity. |
| **Throughput-Batch (TH)** | $1500-3000\text{m}$ | $2048-4096\text{MB}$ | Throughput | Compute-heavy; CFS runqueue saturation. |
| **I/O-Intensive (IO)** | $1000-2000\text{m}$ | $1024-3072\text{MB}$ | IOPS | Memory bus & page cache dirtying. |

---

## 3. Automated Benchmark Execution

```bash
# Execute multi-seed parameter sweep across 5 independent seeds
python benchmark/runner.py

# Execute deterministic CLI benchmark
go run ./cmd/simulator --seed 42 --nodes 6 --workloads 40
```

