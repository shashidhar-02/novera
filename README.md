# Novera (ASTRA) — Autonomous Systems Telemetry & Resource Allocation

[![CI Quality Gate](https://github.com/shashidhar-02/novera/actions/workflows/ci.yml/badge.svg)](https://github.com/shashidhar-02/novera/actions/workflows/ci.yml)
[![Publish Packages](https://github.com/shashidhar-02/novera/actions/workflows/publish.yml/badge.svg)](https://github.com/shashidhar-02/novera/actions/workflows/publish.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/shashidhar-02/novera.svg)](https://pkg.go.dev/github.com/shashidhar-02/novera)
[![Go Report Card](https://goreportcard.com/badge/github.com/shashidhar-02/novera)](https://goreportcard.com/report/github.com/shashidhar-02/novera)
[![Architecture RFC](https://img.shields.io/badge/RFC-0001--Architecture-orange.svg)](rfcs/RFC-0001-architecture.md)
[![GHCR Scheduler](https://img.shields.io/badge/GHCR-novera--scheduler-blue?logo=docker)](https://github.com/shashidhar-02/novera/pkgs/container/novera-scheduler)
[![GHCR Agent](https://img.shields.io/badge/GHCR-novera--agent-blue?logo=docker)](https://github.com/shashidhar-02/novera/pkgs/container/novera-agent)

> **A Linux-native, eBPF-aware, SLO-driven distributed scheduler and workload interference mitigation engine.**

---

## Executive Summary

Modern cloud orchestration frameworks (e.g., standard Kubernetes `kube-scheduler`, Nomad) assign tasks based purely on static resource declarations:

$$\text{Allocatable}(N) = \text{Capacity}(N) - \sum_{W \in \text{Assigned}(N)} \text{Requests}(W)$$

This static abstraction assumes that co-located containers are performance-isolated. In reality:

1. **Microarchitectural Contention:** Co-located workloads silently contend for unmanaged hardware paths: CPU L3 cache lines, memory bus bandwidth, and kernel runqueues.
2. **Hidden Latency Stalls:** Latency-critical workloads experience severe tail latency spikes ($p99$ degradation of 300%–1000%) caused by CPU runqueue starvation and CFS quota throttling—even when average CPU utilization remains well below 50%.
3. **Open-Loop Placement:** Traditional schedulers never observe kernel-level degradation or feed runtime performance metrics back into future placement decisions.

**Novera (ASTRA)** replaces open-loop resource counting with a **closed-loop feedback architecture**. It combines Linux cgroups v2 statistics, Pressure Stall Information (PSI - `/proc/pressure`), and kernel tracepoint eBPF probes (`sched:sched_switch`) to steer latency-sensitive workloads away from noisy neighbors, preserving strict $p99$ SLO commitments.

---

## System Architecture

```text
                               +-------------------------------------+
                               |      Workload Submission API        |
                               |  (Kubernetes Pod / Standalone Job)  |
                               +------------------+------------------+
                                                  |
                                                  v
+---------------------------------------------------------------------------------------------------+
|                                      NOVERA CONTROL PLANE                                         |
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
|         NOVERA NODE AGENT        |         |         NOVERA NODE AGENT        |
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

## Core Innovations & Technical Pillars

### 1. Linux Pressure Stall Information (PSI) Ingestion
Unlike standard CPU utilization metrics which average over long smoothing windows, Linux PSI measures real system queuing delay across CPU, memory, and I/O. Novera penalizes nodes experiencing high `avg10` or `avg60` pressure stalls, preventing latency-critical jobs from landing on saturated nodes.

### 2. eBPF-Powered Runqueue Latency Probes
Using low-overhead eBPF tracepoints hooked to `sched:sched_wakeup` and `sched:sched_switch`, Novera computes kernel-level task waiting times in eBPF BPF-map histograms, emitting per-node $p99$ runqueue latency telemetry directly to userspace without kernel context-switch overhead.

### 3. Empirical Cross-Class Workload Interference Matrix
Novera classifies workloads into distinct profiles:
- **Latency-Sensitive (`LS`):** Web APIs, query engines, payment gateways ($p99 < 25\text{ms}$).
- **Throughput-Batch (`Batch`):** Data processing, machine learning training, background ETLs.
- **I/O-Intensive (`IO`):** Storage engines, message brokers, caching nodes.

The scheduler computes a dynamic affinity penalty:
$$S_{\text{interf}}(N, W) = 100.0 - \sum_{W' \in \text{Active}(N)} \mathcal{M}_{\text{interf}}(C(W), C(W'))$$
preventing batch workloads from monopolizing L3 cache and exhausting memory channels needed by latency-sensitive containers.

### 4. Deterministic Fail-Safe Placement Invariants
If an agent fails, a network partition occurs, or telemetry exceeds a 5-second freshness deadline, Novera automatically degrades node scores to conservative static capacity bounds with an explicit fallback audit trace. No telemetry dropout or clock skew can stall placement.

### 5. High Availability via Kubernetes Leases
Control plane instances operate in an active-standby configuration utilizing Kubernetes Lease primitives (`coordination.k8s.io/leases`). Split-brain hazards are mitigated through atomic compare-and-swap renewal intervals.

---

## Comparative Matrix

| Capability | Upstream `kube-scheduler` | Trimaran (TargetLoadPacking) | Volcano / YuniKorn | Novera (ASTRA) |
|---|---|---|---|---|
| **Placement Telemetry** | Static requests/limits | Delayed Prometheus polling | Queue quotas & gang groups | Real-time Linux PSI + eBPF |
| **Runqueue Contention** | Blind | Blind | Blind | Measured via eBPF `sched_switch` |
| **Microarchitectural Contention**| None | None | None | Empirical Workload Matrix |
| **SLO Violation Guard** | Reactive OOM evictions | Reactive CPU packing | Priority preemption only | Proactive tail latency protection |
| **Decision Explainability** | Opaque event logs | None | Batch queue metrics | Detailed candidate score breakdowns |
| **Telemetry Age Guard** | N/A | None | None | Bounded 5.0s stale degradation |

---

## Quickstart & Local Evaluation

### Option 1: Run the Go Deterministic Simulator
Evaluate Novera's multi-objective interference engine against baseline static placement on a simulated 6-node cluster:

```bash
# Clone the repository
git clone https://github.com/shashidhar-02/novera.git
cd novera

# Run the deterministic simulator CLI
go run ./cmd/simulator --seed 42 --nodes 6 --workloads 40
```

#### Output:
```text
========================================================================
  NOVERA (ASTRA) — Deterministic Workload Interference Simulator v0.1.0
========================================================================
Seed: 42 | Nodes: 6 | Workloads: 40

Metric                         | Baseline (Static)| Novera (Dynamic)
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
Outcome: Novera reduced SLO violations by 78.6%
```

### Option 2: Run the Dual-Engine Mathematical Cross-Check
Verify model mathematical convergence independently of Go via Python:

```bash
# Dual-engine mathematical consistency check
python scripts/verify_model.py

# Multi-seed statistical benchmark runner (5 distinct seeds)
python benchmark/runner.py
```

### Option 3: Run the Operator Diagnostics CLI
Inspect cluster state, telemetry freshness, and decision reasoning:

```bash
# Build the CLI
go build -o bin/astra ./cmd/astra

# Triage cluster telemetry and scheduler state
./bin/astra status
./bin/astra node status
./bin/astra scheduler status

# Explain a placement decision with full candidate score breakdowns
./bin/astra decision explain

# Validate workload profiles against v1alpha1 schemas
./bin/astra policy validate

# Generate a sanitized diagnostic bundle
./bin/astra debug bundle
```

---

## Container Images & Packaging

Production-ready, multi-arch container images are built and published to the **GitHub Container Registry (GHCR)**:

| Component | Image Reference | Target Platforms | Security Profile |
|---|---|---|---|
| **Scheduler** | `ghcr.io/shashidhar-02/novera-scheduler:latest` | `linux/amd64`, `linux/arm64` | Distroless Non-Root (UID 10001) |
| **Node Agent** | `ghcr.io/shashidhar-02/novera-agent:latest` | `linux/amd64`, `linux/arm64` | `CAP_BPF`, `CAP_PERFMON` |
| **Operator CLI** | `ghcr.io/shashidhar-02/novera-cli:latest` | `linux/amd64`, `linux/arm64` | Distroless Non-Root (UID 10001) |

### Pull and Inspect via Docker / Podman
```bash
docker pull ghcr.io/shashidhar-02/novera-scheduler:latest
docker pull ghcr.io/shashidhar-02/novera-agent:latest
docker pull ghcr.io/shashidhar-02/novera-cli:latest
```

---

## Kubernetes Production Deployment

The `deploy/kubernetes/` directory provides zero-trust, enterprise-grade deployment manifests conforming to **Pod Security Standards: Restricted**:

```bash
# 1. Create astra-system namespace with PSA Restricted enforcement
kubectl apply -f deploy/kubernetes/namespace.yaml

# 2. Register WorkloadProfile Custom Resource Definition
kubectl apply -f deploy/kubernetes/crd-workloadprofile.yaml

# 3. Apply least-privilege RBAC policies and NetworkPolicies
kubectl apply -f deploy/kubernetes/rbac.yaml
kubectl apply -f deploy/kubernetes/networkpolicy.yaml

# 4. Deploy High-Availability Scheduler control plane (2 replicas)
kubectl apply -f deploy/kubernetes/scheduler-deployment.yaml

# 5. Deploy Linux Node Agent DaemonSet (attaching PSI and cgroups)
kubectl apply -f deploy/kubernetes/node-agent-daemonset.yaml
```

### Pod Security Hardening Guarantees
- **Non-Root Execution:** `runAsNonRoot: true`, `runAsUser: 10001`, `runAsGroup: 10001`.
- **Read-Only Filesystem:** `readOnlyRootFilesystem: true`.
- **Seccomp Profile:** `type: RuntimeDefault`.
- **Capabilities Dropped:** `drop: ["ALL"]` on scheduler; node agent restricted to `CAP_BPF` and `CAP_PERFMON` (no root/`CAP_SYS_ADMIN`).
- **Network Isolation:** `DefaultDenyAll` ingress with strictly scoped egress to API server and DNS.

---

## Repository Structure

```text
novera/
├── cmd/
│   ├── astra/                  # Operator triage & diagnostic CLI
│   ├── astra-scheduler/        # High-availability control-plane daemon
│   ├── astra-agent/            # Linux PSI & cgroup telemetry node agent
│   └── simulator/              # Deterministic cluster simulation engine
├── pkg/
│   ├── model/                  # Core types (Workload, Node, Telemetry) + RWMutex
│   ├── interference/           # Cross-class contention graph & penalty matrix
│   ├── scheduler/              # Filter/Score engine with clock-skew fallback
│   ├── agent/                  # Linux /proc/pressure & cgroups v2 collectors
│   ├── coordination/           # Active-standby leader election via K8s Leases
│   └── simulator/              # Cluster simulation & validation harness
├── ebpf/
│   ├── sched_switch.bpf.c      # CO-RE BPF program for sched_switch latency
│   └── README.md               # eBPF verifier & capability boundaries
├── deploy/
│   ├── docker/                 # Production hardened multi-stage Dockerfiles
│   │   ├── Dockerfile.scheduler
│   │   ├── Dockerfile.agent
│   │   └── Dockerfile.cli
│   └── kubernetes/             # Production PSA Restricted manifests
│       ├── namespace.yaml
│       ├── crd-workloadprofile.yaml
│       ├── rbac.yaml
│       ├── networkpolicy.yaml
│       ├── scheduler-deployment.yaml
│       └── node-agent-daemonset.yaml
├── rfcs/                       # Formal RFC Architectural Specifications
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
├── docs/                       # Verification Reports & Operations Guides
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
├── .github/workflows/
│   ├── ci.yml                  # Go vet, race detection, unit/fuzz tests
│   └── publish.yml             # Multi-arch GHCR image publishing & binary releases
├── ARCHITECTURE.md             # Complete architectural reference
├── CONTRIBUTING.md             # Developer guidelines & commit standards
├── SECURITY.md                 # Vulnerability reporting & security policy
├── GOVERNANCE.md               # Project governance & RFC lifecycle
├── MAINTAINERS.md              # Active maintainers & review authority
├── SUPPORT.md                  # Community support channels
├── CHANGELOG.md                # Semantic versioning release log
├── LICENSE                     # Apache 2.0 License
└── README.md
```

---

## Technical Specifications & Formal RFCs

| RFC | Title | Subsystem Focus |
|---|---|---|
| [RFC-0001](rfcs/RFC-0001-architecture.md) | High-Level Architecture | Control plane topology, node agent boundary, dataflow |
| [RFC-0002](rfcs/RFC-0002-interference-model.md) | Workload Interference Model | Mathematical contention matrix, classification, penalties |
| [RFC-0003](rfcs/RFC-0003-scheduler.md) | Multi-Objective Scoring Engine | Normalization, tie-breaking, stale telemetry degradation |
| [RFC-0004](rfcs/RFC-0004-linux-agent.md) | Linux Node Agent Architecture | Virtual filesystem parsers (`/proc/pressure`, cgroups v2) |
| [RFC-0005](rfcs/RFC-0005-ebpf.md) | eBPF Runqueue Latency Probe | CO-RE tracepoint hooks, memory bounds, safety |
| [RFC-0006](rfcs/RFC-0006-kubernetes-integration.md) | Kubernetes Integration | PSA Restricted manifests, Custom Resource Definitions |
| [RFC-0007](rfcs/RFC-0007-security-model.md) | Security Architecture | Threat modeling, STRIDE analysis, cryptographic identity |
| [RFC-0008](rfcs/RFC-0008-api-versioning.md) | API Versioning & Evolution | Semantic Versioning, CRD deprecation windows |
| [RFC-0009](rfcs/RFC-0009-ha.md) | High Availability & Coordination | Kubernetes Lease leader election, split-brain mitigation |
| [RFC-0010](rfcs/RFC-0010-release.md) | Release & Packaging Lifecycle | Container publishing, binary distribution, SLSA readiness |

---

## Security & Verification Reports

- [Final Enterprise Verification Report](docs/final-verification-report.md)
- [Claim Verification & Proof Matrix](docs/claim-verification.md)
- [Threat Model & Security Self-Assessment](docs/threat-model.md)
- [Empirical Multi-Seed Benchmark Report](docs/benchmark-report.md)
- [Operator Runbook & Deployment Guide](docs/operations.md)
- [Troubleshooting & Triage Manual](docs/troubleshooting.md)

---

## License

Novera (ASTRA) is licensed under the [Apache License, Version 2.0](LICENSE).
