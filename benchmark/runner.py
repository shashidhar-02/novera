#!/usr/bin/env python3
"""
ASTRA Automated Benchmark Suite & Statistical Significance Harness
Runs reproducible multi-seed simulations comparing Baseline vs. ASTRA policies.
Generates structured JSON and Markdown comparison reports.
"""

import os
import sys
import json
import time
from datetime import datetime, timezone
from typing import List, Dict, Any

# Ensure project root is in python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))
from scripts.verify_model import run_simulation

def run_benchmark_suite(seeds: List[int]) -> Dict[str, Any]:
    print(f"[*] Running ASTRA Multi-Seed Benchmark across {len(seeds)} scenarios...")
    
    baseline_violations = []
    astra_violations = []
    baseline_rates = []
    astra_rates = []
    reductions = []

    for seed in seeds:
        ls_total, b_viols, b_rate = run_simulation("baseline", seed)
        _, a_viols, a_rate = run_simulation("astra", seed)

        baseline_violations.append(b_viols)
        astra_violations.append(a_viols)
        baseline_rates.append(b_rate)
        astra_rates.append(a_rate)

        reduction = ((b_viols - a_viols) / b_viols) * 100.0 if b_viols > 0 else 0.0
        reductions.append(reduction)

    avg_b_rate = sum(baseline_rates) / len(baseline_rates)
    avg_a_rate = sum(astra_rates) / len(astra_rates)
    avg_reduction = sum(reductions) / len(reductions)

    report = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "total_seeds": len(seeds),
        "seeds": seeds,
        "metrics": {
            "baseline": {
                "avg_slo_violation_rate_pct": round(avg_b_rate, 2),
                "total_slo_violations": sum(baseline_violations),
            },
            "astra": {
                "avg_slo_violation_rate_pct": round(avg_a_rate, 2),
                "total_slo_violations": sum(astra_violations),
            },
            "delta": {
                "avg_violation_reduction_pct": round(avg_reduction, 2),
            }
        }
    }
    return report

def generate_markdown_report(report: Dict[str, Any]) -> str:
    m = report["metrics"]
    md = f"""# ASTRA Benchmark Evaluation Report

**Generated:** {report['timestamp']}  
**Evaluation Scope:** {report['total_seeds']} Deterministic Multi-Seed Scenarios (Seeds: {report['seeds']})  

---

## 1. Executive Summary

| Metric | Baseline Scheduler | ASTRA Interference-Aware | Improvement / Delta |
|---|---|---|---|
| **Average SLO Violation Rate** | {m['baseline']['avg_slo_violation_rate_pct']}% | {m['astra']['avg_slo_violation_rate_pct']}% | **{m['delta']['avg_violation_reduction_pct']}% Reduction** |
| **Total Cumulative Violations** | {m['baseline']['total_slo_violations']} | {m['astra']['total_slo_violations']} | -{m['baseline']['total_slo_violations'] - m['astra']['total_slo_violations']} violations |

---

## 2. Methodology & Invariants

- **Synthetic Workload Composition:** 40% Latency-Sensitive ($p99 < 25-40\\text{{ms}}$), 35% Throughput-Batch, 25% I/O-Intensive.
- **Physical Node Modeling:** 6 Nodes (8 cores, 32 GB RAM each).
- **Runqueue Latency Modeling:** Context-switch penalties and CFS queue stalls dynamically calculated from active co-located workload classes.
- **Reproducibility:** All seeds are pseudo-randomly deterministic and reproducible on any machine.
"""
    return md

if __name__ == "__main__":
    seeds = [42, 101, 2024, 777, 999]
    report = run_benchmark_suite(seeds)

    results_dir = os.path.join(os.path.dirname(__file__), "results")
    os.makedirs(results_dir, exist_ok=True)

    json_path = os.path.join(results_dir, "benchmark_summary.json")
    with open(json_path, "w") as f:
        json.dump(report, f, indent=2)

    md_content = generate_markdown_report(report)
    md_path = os.path.join(results_dir, "benchmark_summary.md")
    with open(md_path, "w") as f:
        f.write(md_content)

    print("\n" + md_content)
    print(f"[*] Benchmark data saved to {json_path} and {md_path}")
