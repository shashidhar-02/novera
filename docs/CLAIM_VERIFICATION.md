# ASTRA Public Claim Verification Matrix

**Audit Standard:** Normative Source Baseline & Zero-Assumption Rule  
**Audit Date:** 2026-11-03  
**Status:** Complete  

---

## 1. Claim Verification Summary

| Public Claim | Normative Source & Implementation Evidence | Verification Status | Notes / Limitations |
|---|---|---|---|
| **"ASTRA uses continuous Linux telemetry (cgroups v2 & PSI) to inform scheduling."** | [`pkg/agent/psi.go`](../pkg/agent/psi.go), [`pkg/agent/cgroup.go`](../pkg/agent/cgroup.go), [`pkg/agent/collector.go`](../pkg/agent/collector.go) | **VERIFIED** | Parsers and collector implemented and unit tested against standard Linux 5.8+ `/proc/pressure` and `/sys/fs/cgroup` formats. |
| **"ASTRA measures in-kernel runqueue latency using eBPF tracepoints."** | [`ebpf/sched_switch.bpf.c`](../ebpf/sched_switch.bpf.c), [`ebpf/README.md`](../ebpf/README.md) | **PARTIALLY VERIFIED** | Valid C CO-RE source code with hash maps and ring buffer exists; compiled bytecode execution validated in Linux CI environment, not on bare-metal Windows host. |
| **"ASTRA provides deterministic, explainable scheduling decisions."** | [`pkg/scheduler/engine.go`](../pkg/scheduler/engine.go), [`pkg/scheduler/engine_test.go`](../pkg/scheduler/engine_test.go) | **VERIFIED** | Proven via unit tests; every decision outputs candidate scores, rejection reasons, and dominant pressure factors. |
| **"ASTRA safely degrades when telemetry is stale or skewed."** | `TestNodeTelemetry_StalenessAndClockSkew`, `TestEngine_StaleTelemetryFallback` | **VERIFIED** | When telemetry is older than 5s or in the future, pressure score defaults to neutral (50.0) and transitions to capacity-based placement. |
| **"ASTRA reduces SLO tail-latency violations under noisy-neighbor contention."** | [`benchmark/results/benchmark_summary.md`](../benchmark/results/benchmark_summary.md), [`scripts/verify_model.py`](../scripts/verify_model.py) | **VERIFIED** | Tested across 5 multi-seed deterministic scenarios; ASTRA consistently steers Latency-Sensitive workloads away from Throughput-Batch workloads. |
| **"ASTRA is compliant with Kubernetes Pod Security Standards: Restricted."** | [`deploy/kubernetes/scheduler-deployment.yaml`](../deploy/kubernetes/scheduler-deployment.yaml), [`deploy/kubernetes/namespace.yaml`](../deploy/kubernetes/namespace.yaml) | **VERIFIED** | Manifests declare `runAsNonRoot: true`, `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, `drop: [ALL]`, and `seccompProfile: RuntimeDefault`. |
| **"ASTRA is a drop-in production replacement for kube-scheduler today."** | N/A | **FALSE (REJECTED)** | ASTRA is an out-of-tree secondary scheduler / plugin prototype; calling it a complete drop-in production replacement is false. Documentation explicitly amended. |
| **"ASTRA implements causal inference in the kernel."** | N/A | **FALSE (REJECTED)** | ASTRA implements empirical correlation and cross-class affinity scoring, not formal causal discovery DAGs. Documentation explicitly amended. |

