# ASTRA Repository Assessment

**Document ID:** ASTRA-ASSESS-001  
**Date:** 2026-11-03  
**Status:** Approved / Baseline  
**Scope:** Initial workspace, host environment, toolchain capabilities, hardware constraints, and development strategy.

---

## 1. Executive Summary

This assessment audits the initial state of the `ASTRA` workspace at `d:\astra`. The mission is to establish a systems-level, open-source distributed scheduler and interference-mitigation engine (**ASTRA — Autonomous Systems Telemetry & Resource Allocation**).

The project is governed by strict engineering principles:
- No premature feature bloat or hollow "AI-wrapper" patterns.
- No fabricated benchmark numbers or simulated production claims.
- Rigorous architecture-first engineering (RFC-driven design).
- Deterministic simulation and unit testing before multi-node distributed complexity.
- Progressive evolution: Simulator -> Linux node prototype (cgroups v2 + PSI + eBPF) -> Kubernetes Scheduling Framework plugin.

---

## 2. Host Environment & Hardware Constraints

An audit of the local development environment reveals the following constraints and characteristics:

| Property | Value / Status | Implication |
|---|---|---|
| **Host OS** | Windows 11 / Windows NT (PowerShell) | Kernel-level eBPF cannot execute natively on the Windows host kernel without Linux emulation or a Linux VM/runner. |
| **Hardware Virtualization** | Disabled in UEFI/BIOS (WSL2 disabled) | Direct local WSL2 / VM execution is currently inactive until firmware virtualization is enabled or remote Linux testbeds are provisioned. |
| **Available RAM** | ~8 GB host budget | High resource constraint. Simultaneous execution of Kubernetes (k3s/kind), Prometheus, Loki, OpenTelemetry, Jaeger, and multi-node clusters locally will trigger severe memory pressure and swap thrashing. |
| **Installed Runtimes** | Python 3.14.6, Git 2.55.0 | High-performance scripting, data processing, and validation available out-of-the-box. |
| **Container Engine** | Docker Desktop directory present, CLI inactive | Containerized testing and CI pipelines must be lightweight or delegated to GitHub Actions / remote testbeds. |
| **Systems Languages** | Go / Rust / Clang | Control plane and scheduler logic must be authored in standard, portable Go (to be installed/configured), with deterministic unit tests and cross-platform simulation mode decoupled from Linux kernel headers. |

---

## 3. Existing Workspace Assets

- `ASTRA_ENTERPRISE_PROJECT.md`: Comprehensive enterprise specification outlining:
  - System architecture (Control Plane, Scheduler, Node Agent, eBPF telemetry, Simulator).
  - Theoretical framework for causal interference modeling vs. naive correlation.
  - SLO-aware multi-objective optimization equations.
  - Security boundaries, least-privilege eBPF capabilities (`CAP_BPF`, `CAP_PERFMON`).
  - Benchmarking methodology and verification standards.

---

## 4. Architectural Separation Strategy

To ensure rapid, reliable, and testable engineering despite the 8 GB host and non-Linux host environment, ASTRA enforces strict architectural decoupling:

```text
+-----------------------------------------------------------------------+
|                         ASTRA Control Plane                           |
|  - Scheduler Engine (Candidate filtering, scoring, tie-breaking)      |
|  - Interference Matrix & Graph                                        |
|  - SLO-Violation Predictor & Policy Evaluator                         |
|  - Portable Go / Cross-Platform Core (Runs on Windows, Linux, Darwin) |
+-----------------------------------------------------------------------+
                                  |
         +------------------------+------------------------+
         |                                                 |
         v                                                 v
+-------------------------------+       +-------------------------------+
|      ASTRA Simulator          |       |       ASTRA Node Agent        |
|  - Synthetic Telemetry Gen    |       |  - Real Linux Node Telemetry  |
|  - Deterministic PRNG Seed    |       |  - cgroups v2 (`/sys/fs/...`) |
|  - Noisy-Neighbor Injection   |       |  - PSI (`/proc/pressure/...`) |
|  - Benchmarking Harness       |       |  - eBPF Probes (CO-RE/libbpf) |
|  (Runs anywhere, zero kernel) |       |  (Requires Linux Kernel 5.8+) |
+-------------------------------+       +-------------------------------+
```

### Stage 1: Deterministic Engine & Simulator
- 100% portable Go implementation.
- Zero external kernel or container runtime dependencies.
- Simulates workload execution, burst profiles, CPU/Memory/IO resource starvation, and noisy-neighbor interference scenarios.
- Deterministic verification with reproducible random seeds.

### Stage 2: Linux Telemetry & eBPF Subsystem
- Developed using standard C / CO-RE (Compile Once – Run Everywhere) targeting Linux 5.8+.
- Node Agent written in Go/Rust with fallback to standard `/proc/pressure` (PSI) and `/sys/fs/cgroup/` when eBPF is disabled or restricted.
- Tested and verified in Linux CI environments (GitHub Actions Ubuntu 22.04/24.04 runners with full kernel access).

### Stage 3: Kubernetes Scheduling Framework Integration
- Implement `Score`, `Filter`, `PreScore`, and `Reserve` plugins adhering to the upstream `k8s.io/kube-scheduler` plugin interfaces.
- Safe fallback to default scheduler behavior in the event of missing or stale telemetry.

---

## 5. Toolchain & Development Plan

1. **Toolchain Setup**:
   - Provision Go (1.23+ or 1.24+) for core scheduler, simulation harness, and controller development.
   - Configure formatting (`gofmt`), linting (`golangci-lint`), and testing (`go test -race`).
2. **Phase 1 Deliverables**:
   - `artifacts/repository-assessment.md` (This document)
   - `artifacts/prior-art.md` (Deep technical critique of Borg, K8s default scheduler, Volcano, Trimaran, Caladan, and Bubble-Up)
   - `rfcs/RFC-0001-architecture.md` (Formal system architecture RFC)
   - Minimal Deterministic Simulator: Core scheduler data models, hard constraints, scoring pipeline, synthetic interference matrices, and benchmark CLI.
