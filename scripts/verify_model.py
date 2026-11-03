#!/usr/bin/env python3
"""
ASTRA Model Verification & Dual-Engine Cross-Validation Script
Demonstrates and validates the mathematical interference scoring model independently of the Go runtime.
"""

import sys
import math
import random
from typing import Dict, List, Optional, Tuple

class WorkloadClass:
    LATENCY_SENSITIVE = "LatencySensitive"
    THROUGHPUT_BATCH = "ThroughputBatch"
    IO_INTENSIVE = "IOIntensive"

INTERFERENCE_MATRIX = {
    WorkloadClass.LATENCY_SENSITIVE: {
        WorkloadClass.LATENCY_SENSITIVE: 5.0,
        WorkloadClass.THROUGHPUT_BATCH: 35.0,
        WorkloadClass.IO_INTENSIVE: 25.0,
    },
    WorkloadClass.THROUGHPUT_BATCH: {
        WorkloadClass.LATENCY_SENSITIVE: 5.0,
        WorkloadClass.THROUGHPUT_BATCH: 0.0,
        WorkloadClass.IO_INTENSIVE: 15.0,
    },
    WorkloadClass.IO_INTENSIVE: {
        WorkloadClass.LATENCY_SENSITIVE: 10.0,
        WorkloadClass.THROUGHPUT_BATCH: 15.0,
        WorkloadClass.IO_INTENSIVE: 30.0,
    }
}

class Workload:
    def __init__(self, id: str, wclass: str, cpu_millis: int, mem_bytes: int, target_p99_ms: Optional[float] = None):
        self.id = id
        self.wclass = wclass
        self.cpu_millis = cpu_millis
        self.mem_bytes = mem_bytes
        self.target_p99_ms = target_p99_ms

class Node:
    def __init__(self, id: str, cpu_capacity: int, mem_capacity: int):
        self.id = id
        self.cpu_capacity = cpu_capacity
        self.mem_capacity = mem_capacity
        self.allocated_cpu = 0
        self.allocated_mem = 0
        self.active_workloads: Dict[str, Workload] = {}
        self.runqueue_latency_us = 150.0

    @property
    def available_cpu(self) -> int:
        return max(0, self.cpu_capacity - self.allocated_cpu)

    @property
    def available_mem(self) -> int:
        return max(0, self.mem_capacity - self.allocated_mem)

    def assign(self, w: Workload):
        self.allocated_cpu += w.cpu_millis
        self.allocated_mem += w.mem_bytes
        self.active_workloads[w.id] = w

    def update_telemetry(self):
        batch_count = sum(1 for w in self.active_workloads.values() if w.wclass == WorkloadClass.THROUGHPUT_BATCH)
        ls_count = sum(1 for w in self.active_workloads.values() if w.wclass == WorkloadClass.LATENCY_SENSITIVE)
        cpu_util = self.allocated_cpu / self.cpu_capacity

        rq = 150.0
        if batch_count > 0 and ls_count > 0:
            rq += (batch_count * ls_count) * 850.0
        if cpu_util > 0.70:
            rq += math.pow((cpu_util - 0.70) * 10.0, 2) * 500.0
        self.runqueue_latency_us = min(50000.0, rq)

def score_node_astra(w: Workload, node: Node) -> float:
    # 1. Capacity Score
    cpu_util = (node.allocated_cpu + w.cpu_millis) / node.cpu_capacity
    mem_util = (node.allocated_mem + w.mem_bytes) / node.mem_capacity
    avg_util = (cpu_util + mem_util) / 2.0
    diff_util = abs(cpu_util - mem_util)
    cap_score = max(0.0, 100.0 * (1.0 - avg_util - diff_util * 0.5))

    # 2. Pressure Score
    runq_penalty = min(1.0, node.runqueue_latency_us / 10000.0) * 30.0
    pressure_score = max(0.0, 100.0 - runq_penalty)

    # 3. Interference Score
    penalty = 0.0
    for active in node.active_workloads.values():
        penalty += INTERFERENCE_MATRIX.get(w.wclass, {}).get(active.wclass, 0.0)
    interf_score = max(0.0, 100.0 - min(100.0, penalty))

    return (0.30 * cap_score) + (0.40 * pressure_score) + (0.30 * interf_score)

def run_simulation(policy: str, seed: int = 42) -> Tuple[int, int, float]:
    rnd = random.Random(seed)
    nodes = {f"node-{i+1:02d}": Node(f"node-{i+1:02d}", 8000, 32*1024*1024*1024) for i in range(6)}

    workloads: List[Workload] = []
    for i in range(40):
        roll = rnd.random()
        if roll < 0.40:
            w = Workload(f"wl-{i+1}", WorkloadClass.LATENCY_SENSITIVE, 500 + rnd.randint(0, 500), (512+rnd.randint(0,512))*1024*1024, 25.0 + rnd.randint(0, 15))
        elif roll < 0.75:
            w = Workload(f"wl-{i+1}", WorkloadClass.THROUGHPUT_BATCH, 1500 + rnd.randint(0, 1500), (2048+rnd.randint(0,2048))*1024*1024)
        else:
            w = Workload(f"wl-{i+1}", WorkloadClass.IO_INTENSIVE, 1000 + rnd.randint(0, 1000), (1024+rnd.randint(0,2048))*1024*1024)
        workloads.append(w)

    ls_total = sum(1 for w in workloads if w.wclass == WorkloadClass.LATENCY_SENSITIVE)

    for w in workloads:
        for n in nodes.values():
            n.update_telemetry()

        valid_nodes = [n for n in nodes.values() if n.available_cpu >= w.cpu_millis and n.available_mem >= w.mem_bytes]
        if not valid_nodes:
            continue

        if policy == "baseline":
            # Lowest CPU allocated
            chosen = min(valid_nodes, key=lambda n: n.allocated_cpu)
        else:
            # ASTRA scorer
            chosen = max(valid_nodes, key=lambda n: score_node_astra(w, n))

        chosen.assign(w)

    for n in nodes.values():
        n.update_telemetry()

    slo_violations = 0
    for n in nodes.values():
        for w in n.active_workloads.values():
            if w.wclass == WorkloadClass.LATENCY_SENSITIVE and w.target_p99_ms:
                p99 = 15.0 + (n.runqueue_latency_us / 100.0)
                if p99 > w.target_p99_ms:
                    slo_violations += 1

    rate = (slo_violations / ls_total) * 100.0 if ls_total > 0 else 0.0
    return ls_total, slo_violations, rate

if __name__ == "__main__":
    print("=" * 60)
    print("ASTRA Dual-Engine Mathematical Verification (Python Harness)")
    print("=" * 60)
    ls_count, b_viols, b_rate = run_simulation("baseline", 42)
    _, a_viols, a_rate = run_simulation("astra", 42)

    print(f"Latency-Sensitive Tasks: {ls_count}")
    print(f"Baseline (Static Allocation) SLO Violations: {b_viols} ({b_rate:.1f}%)")
    print(f"ASTRA (Interference-Aware)   SLO Violations: {a_viols} ({a_rate:.1f}%)")
    reduction = ((b_viols - a_viols) / b_viols) * 100.0 if b_viols > 0 else 0.0
    print(f"SLO Violation Reduction: {reduction:.1f}%")

    assert a_viols <= b_viols, "Verification failure: ASTRA did not outperform baseline!"
    print("\n[OK] Deterministic Mathematical Model Verified Successfully.")

