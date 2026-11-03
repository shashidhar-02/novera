# ASTRA Architecture Guide

> **Closed-loop, kernel-informed workload scheduling driven by Linux runtime telemetry.**

---

## 1. Subsystem Decomposition

ASTRA is architected into five decoupled layers to ensure that the scheduler core remains independently testable without requiring physical kernel probes or a live Kubernetes cluster:

```text
+-------------------------------------------------------------------------------+
|                            Kubernetes Control Plane                           |
|       (CRD: WorkloadProfiles, API Server, Leases, Scheduling Framework)       |
+---------------------------------------+---------------------------------------+
                                        |
                                        v
+-------------------------------------------------------------------------------+
|                        ASTRA Scheduling Engine (`pkg/scheduler`)              |
|  - Candidate Filter (Hard Capacity & Isolation Constraints)                   |
|  - Multi-Objective Scorer (Capacity, Pressure Stall Penalties, Interference)  |
|  - Explainable Audit Emitter & Deterministic Tie-Breaker                      |
+---------------------------------------+---------------------------------------+
                                        ^
                                        | (mTLS Normalized Telemetry Stream)
                                        |
+-------------------------------------------------------------------------------+
|                        ASTRA Node Agent (`pkg/agent`)                         |
|  - Linux PSI Ingestion (`/proc/pressure/{cpu,memory,io}`)                     |
|  - cgroups v2 Throttling Tracker (`/sys/fs/cgroup/.../cpu.stat`)              |
|  - Monotonic Sequencer & Clock-Skew Protection                                |
+---------------------------------------+---------------------------------------+
                                        |
                                        v
+-------------------------------------------------------------------------------+
|                        eBPF Kernel Probes (`ebpf/`)                           |
|  - Tracepoints: sched/sched_wakeup, sched/sched_switch                        |
|  - Wakeup-to-Schedule Runqueue Latency Histograms (< 8MB BPF Maps)            |
|  - Non-blocking Ring Buffer Anomaly Stream (> 10ms stalls)                    |
+-------------------------------------------------------------------------------+
```

---

## 2. Component Directory Map

- [`pkg/model/`](pkg/model/): Core data types (`Workload`, `Node`, `NodeTelemetry`, `PSIPressure`) with `sync.RWMutex` concurrency protections.
- [`pkg/interference/`](pkg/interference/): Empirical cross-class contention model with diminishing marginal penalties.
- [`pkg/scheduler/`](pkg/scheduler/): Filter and multi-objective scoring engine with clock-skew fallback safeguards.
- [`pkg/agent/`](pkg/agent/): Linux virtual filesystem parsers for PSI and cgroups v2.
- [`pkg/coordination/`](pkg/coordination/): Distributed Lease-based active-standby leader election.
- [`pkg/simulator/`](pkg/simulator/): Deterministic cluster simulation engine.
- [`ebpf/`](ebpf/): BPF CO-RE C programs for in-kernel runqueue wait tracking.
- [`cmd/`](cmd/):
  - `cmd/astra/`: Operator diagnostics CLI.
  - `cmd/astra-scheduler/`: Control-plane daemon with HTTP health probes.
  - `cmd/astra-agent/`: Linux node telemetry daemon.
  - `cmd/simulator/`: Deterministic benchmark runner.
- [`deploy/kubernetes/`](deploy/kubernetes/): Pod Security Standards: Restricted compliant manifests, least-privilege RBAC, and default-deny NetworkPolicies.
- [`rfcs/`](rfcs/): Architectural RFCs 0001 through 0010.
- [`docs/`](docs/): Threat models, benchmark reports, security reviews, and verification audits.
