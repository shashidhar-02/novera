# ASTRA Benchmark Evaluation Report

**Generated:** 2026-11-03T15:51:22.241085+00:00  
**Evaluation Scope:** 5 Deterministic Multi-Seed Scenarios (Seeds: [42, 101, 2024, 777, 999])  

---

## 1. Executive Summary

| Metric | Baseline Scheduler | ASTRA Interference-Aware | Improvement / Delta |
|---|---|---|---|
| **Average SLO Violation Rate** | 95.43% | 94.85% | **0.17% Reduction** |
| **Total Cumulative Violations** | 84 | 83 | -1 violations |

---

## 2. Methodology & Invariants

- **Synthetic Workload Composition:** 40% Latency-Sensitive ($p99 < 25-40\text{ms}$), 35% Throughput-Batch, 25% I/O-Intensive.
- **Physical Node Modeling:** 6 Nodes (8 cores, 32 GB RAM each).
- **Runqueue Latency Modeling:** Context-switch penalties and CFS queue stalls dynamically calculated from active co-located workload classes.
- **Reproducibility:** All seeds are pseudo-randomly deterministic and reproducible on any machine.
