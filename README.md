# ASTRA — Autonomous Systems Telemetry & Resource Allocation

[![CI Quality Gate](https://github.com/astra-orchestration/astra/actions/workflows/ci.yml/badge.svg)](https://github.com/astra-orchestration/astra/actions)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/astra-orchestration/astra)](https://goreportcard.com/report/github.com/astra-orchestration/astra)
[![Architecture RFC](https://img.shields.io/badge/RFC-0001--Architecture-orange.svg)](rfcs/RFC-0001-architecture.md)

> **A Linux-native, eBPF-aware, SLO-driven workload scheduler and interference-mitigation engine.**

ASTRA is an open-source distributed systems project designed to explore a fundamental cloud infrastructure problem:

> *Can continuously observed, kernel-level workload interference telemetry improve distributed workload placement and tail-latency SLO adherence compared to static, request-based scheduling?*

ASTRA is **not** a Kubernetes clone, Docker clone, or LLM-based log assistant. It is a feedback-driven scheduling control plane that replaces blind static resource allocations with closed-loop telemetry from Linux cgroups v2, Pressure Stall Information (PSI), and eBPF runqueue probes.

---

## Architecture Overview

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

## Core Innovations

1. **Kernel Pressure Stall Integration (PSI):**
   Standard schedulers only monitor utilization (e.g. CPU % or Memory bytes). ASTRA monitors *pressure stalls* (`/proc/pressure/cpu`, `/proc/pressure/memory`, `/proc/pressure/io`). A node at 50% CPU with 25% runqueue stall is recognized as saturated and penalised for latency-sensitive placements.
2. **eBPF-Assisted Runqueue Histograms:**
   Lightweight eBPF probes hooked to `sched:sched_switch` compute per-cgroup wakeup-to-schedule latency histograms in kernel memory, streaming percentile latencies ($p50$, $p95$, $p99$) locklessly to user space.
3. **Cross-Class Workload Interference Graph:**
   Categorizes tasks into Latency-Sensitive (LS), Throughput-Batch, and I/O-Intensive classes. Penalizes destructive co-locations (e.g. LS co-located with noisy Batch jobs) through an empirical interference matrix.
4. **Deterministic Safety Invariants:**
   If telemetry drops or exceeds a 5-second staleness window, the scheduler automatically transitions to safe capacity fallback, ensuring no partition or agent failure can freeze or corrupt placement.

---

## Comparative Matrix

| Feature | Upstream `kube-scheduler` | Trimaran (TargetLoadPacking) | Volcano / YuniKorn | ASTRA |
|---|---|---|---|---|
| **Placement Input** | Static `requests` & `limits` | Delayed Prometheus averages | Queue quotas & gang groups | Dynamic kernel PSI + eBPF telemetry |
| **Runqueue Contention** | Blind | Blind | Blind | Measured via eBPF `sched_switch` |
| **Interference Modeling**| None | None | None | Empirical Affinity Graph |
| **SLO Violation Guard** | Reactive OOM evictions only | Reactive CPU packing | Priority preemption only | Proactive tail latency protection |
| **Explainable Audit** | Basic event logs | None | Batch queue metrics | Complete per-node scoring breakdown |

---

## Implementation Status & Roadmap

| Subsystem / Tier | Status | Verification & Artifacts |
|---|---|---|
| **Stage 1: Core Engine & Simulator** | **COMPLETE & VERIFIED** | Deterministic simulation, multi-objective scoring, interference matrix, unit/adversarial tests, CI pipeline. |
| **Stage 2: Linux Node Agent (cgroups/PSI/eBPF)** | **PLANNED ROADMAP** | Specification in [RFC-0001](rfcs/RFC-0001-architecture.md); probe implementation targeted for v0.2.0. |
| **Stage 3: Kubernetes Scheduling Plugin** | **PLANNED ROADMAP** | K8s Scheduling Framework plugin targeted for v0.3.0. |

---

## Operational Modes

ASTRA is engineered across three decoupled execution tiers:

1. **Simulator Mode (v0.1.0 - Current):** Pure user-space deterministic simulation running on any OS (Linux, macOS, Windows) without requiring kernel privileges or container engines. Used for algorithmic verification, CI benchmarking, adversarial testing, and regression analysis.
2. **Linux Node Prototype (v0.2.0 - Roadmap):** Standalone control plane and node agent communicating over mTLS, collecting live metrics from Linux 5.8+ cgroups v2 (`/sys/fs/cgroup`) and PSI (`/proc/pressure`).
3. **Kubernetes Integration (v0.3.0 - Roadmap):** Deployment as an out-of-tree secondary scheduler or upstream Kubernetes Scheduling Framework plugin (`Score`, `PreScore`, `Filter`).

---

## Quickstart: Deterministic Simulator

Run the deterministic benchmark suite locally to observe how ASTRA's interference-aware scoring protects latency-sensitive workloads against noisy-neighbor contention:

```bash
# 1. Run the Go simulator
go run ./cmd/simulator --seed 42 --nodes 6 --workloads 40

# 2. Or run the cross-platform dual-engine verification script
python scripts/verify_model.py
```

### Sample Benchmark Output

```text
========================================================================
  ASTRA — Deterministic Workload Interference Simulator v0.1.0
========================================================================
Seed: 42 | Nodes: 6 | Workloads: 40

Metric                         | Baseline (Static)| ASTRA (Dynamic)
------------------------------------------------------------------------
Placed Workloads               | 40              | 40             
Rejected Workloads             | 0               | 0              
Latency-Sensitive Tasks        | 16              | 16             
SLO Violations                 | 14              | 3              
SLO Violation Rate             | 87.5%           | 18.8%          
Avg CPU Allocation             | 68.2%           | 68.2%          
Avg Memory Allocation          | 61.4%           | 61.4%          
Avg Runqueue Latency           | 1420.5 us       | 310.2 us       
Max Runqueue Latency           | 4850.0 us       | 780.0 us       
------------------------------------------------------------------------
Outcome: ASTRA reduced SLO violations by 78.6%
```

---

## Repository Structure

```text
astra/
├── cmd/
│   ├── astra/                # Operator diagnostics & triage CLI
│   ├── astra-scheduler/      # Control-plane daemon with HTTP health probes
│   ├── astra-agent/          # Linux node telemetry daemon
│   └── simulator/            # Deterministic simulation CLI entrypoint
├── pkg/
│   ├── model/                # Core types (Workload, Node, Telemetry, PSI) + RWMutex
│   ├── interference/         # Empirical cross-class contention matrix
│   ├── scheduler/            # Filter & score engine with clock-skew fallback
│   ├── agent/                # Linux PSI & cgroups v2 telemetry collectors
│   ├── coordination/         # Active-standby leader election via Kubernetes Leases
│   └── simulator/            # Deterministic cluster simulation engine
├── ebpf/
│   ├── sched_switch.bpf.c    # CO-RE BPF C program for sched_wakeup & sched_switch
│   └── README.md             # eBPF verifier & capability boundaries
├── deploy/kubernetes/
│   ├── namespace.yaml        # Pod Security Admission: Restricted baseline
│   ├── crd-workloadprofile.yaml # WorkloadProfile CRD
│   ├── rbac.yaml             # Least-privilege ServiceAccount, Role, ClusterRole
│   ├── networkpolicy.yaml    # Default-deny-all with explicit DNS & API server rules
│   ├── scheduler-deployment.yaml # Hardened deployment (non-root, readOnlyRootFS)
│   └── node-agent-daemonset.yaml # DaemonSet with CAP_BPF / CAP_PERFMON
├── rfcs/
│   ├── RFC-0001-architecture.md
│   ├── RFC-0002-interference-model.md
│   ├── RFC-0003-scheduler.md
│   ├── RFC-0004-linux-agent.md
│   ├── RFC-0005-ebpf.md
│   ├── RFC-0006-kubernetes-integration.md
│   ├── RFC-0007-security-model.md
│   ├── RFC-0008-api-versioning.md
│   ├── RFC-0009-ha.md
│   └── RFC-0010-release.md
├── docs/
│   ├── architecture.md
│   ├── threat-model.md
│   ├── security-model.md
│   ├── security-self-assessment.md
│   ├── benchmark-plan.md
│   ├── benchmark-report.md
│   ├── verification-plan.md
│   ├── final-verification-report.md
│   ├── claim-verification.md
│   ├── compatibility.md
│   ├── operations.md
│   └── troubleshooting.md
├── ARCHITECTURE.md           # High-level architecture reference
├── CONTRIBUTING.md           # Engineering guidelines & RFC process
├── CODE_OF_CONDUCT.md        # Contributor Covenant 2.1
├── SECURITY.md               # Vulnerability reporting policy
├── GOVERNANCE.md             # Project governance & RFC lifecycle
├── MAINTAINERS.md            # Active maintainers & review authority
├── SUPPORT.md                # Community support channels
├── CHANGELOG.md              # Semantic Versioning change log
├── LICENSE                   # Apache 2.0 License
└── README.md
```

---

## Documentation & RFC Suite

- [Architecture Reference](ARCHITECTURE.md)
- [RFC Index](rfcs/) (RFC-0001 through RFC-0010)
- [Operator Runbook](docs/operations.md)
- [Troubleshooting & Triage](docs/troubleshooting.md)
- [Compatibility Matrix](docs/compatibility.md)
- [Benchmark Plan & Evaluation](docs/benchmark-plan.md)
- [Threat Model & Security Self-Assessment](docs/threat-model.md)
- [Final Verification & Audit Report](docs/final-verification-report.md)
- [Claim Verification Matrix](docs/claim-verification.md)

---

## Security & Reliability Principles

- **Least Privilege:** Node agent telemetry runs with `CAP_BPF` and `CAP_PERFMON` without broad `CAP_SYS_ADMIN` access.
- **Fail-Safe Placement:** Stale telemetry ($> 5\text{s}$) automatically degrades the node score to conservative static capacity bounds.
- **Explainability:** All decisions are logged with deterministic reasons for node ranking.
- **No Non-Deterministic LLMs in Critical Path:** The placement decision runs in $< 5\text{ms}$ with zero external model calls.

---

## License

ASTRA is licensed under the [Apache License, Version 2.0](LICENSE).

