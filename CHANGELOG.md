# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.0] - 2026-11-03

### Added
- **Core Scheduler Engine:** Implemented multi-objective normalized scoring ($S_{\text{cap}}$, $S_{\text{press}}$, $S_{\text{interf}}$) with deterministic tie-breaking.
- **Interference Matrix:** Empirical cross-class contention model penalizing hostile Latency-Sensitive + Batch co-locations.
- **Linux Agent Telemetry:** `/proc/pressure` (PSI) and cgroups v2 (`cpu.stat`, `memory.current`) parsers and telemetry collector in `pkg/agent`.
- **eBPF Instrumentation:** In-kernel runqueue wait latency tracepoint program (`ebpf/sched_switch.bpf.c`) with BPF hash maps and ring buffer.
- **Kubernetes Hardened Manifests:** Complete deployment suite with `pod-security.kubernetes.io/enforce: restricted`, least-privilege RBAC, default-deny NetworkPolicy, and CRD in `deploy/kubernetes/`.
- **Deterministic Cluster Simulator:** Reproducible workload generator and simulation harness comparing Baseline vs. ASTRA across arbitrary random seeds.
- **Verification Suites:** Unit tests, negative edge case tests, concurrency stress tests (50 concurrent workers), and Python dual-engine validation.
- **RFC Suite:** Formal RFCs 0001 through 0008 governing system architecture, interference, scheduler, Linux agent, eBPF, Kubernetes, security, and API versioning.

### Fixed
- Remediated resource accounting leak on duplicate workload assignment.
- Remediated division-by-zero on nodes with zero capacity.
- Remediated future clock-skew telemetry staleness check bypass.
- Added `sync.RWMutex` to `model.Node` to ensure thread-safe concurrent scheduling and allocation.
