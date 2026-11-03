# ASTRA Comprehensive Threat Model

**Standard:** STRIDE / NIST SP 800-218 (SSDF)  
**Date:** 2026-11-03  
**Status:** Approved  

---

## 1. System Overview & Trust Boundaries

```text
+-------------------------------------------------------------------------------+
|                             Kubernetes API Server                             |
+---------------------------------------+---------------------------------------+
                                        | (mTLS, RBAC authenticated)
                                        v
+-------------------------------------------------------------------------------+
|                       ASTRA Scheduler (Control Plane)                         |
|  - Runs as non-root (UID 10001)                                               |
|  - Read-only root filesystem                                                  |
|  - Bounded CPU/Memory limits                                                  |
|  - Capability Drop: [ALL]                                                     |
+---------------------------------------+---------------------------------------+
                                        ^
                                        | (mTLS stream, authenticated Node ID)
                                        |
+---------------------------------------+---------------------------------------+
|                    ASTRA Node Agent (DaemonSet)                               |
|  - hostPID: true (Required for cgroup translation)                            |
|  - Capabilities: Drop [ALL], Add [BPF, PERFMON]                               |
|  - Read-only virtual mounts: /sys/fs/cgroup, /proc/pressure                   |
+---------------------------------------+---------------------------------------+
                                        |
                                        v
+-------------------------------------------------------------------------------+
|                    Linux Kernel Space (eBPF Tracepoints)                      |
|  - Read-only hooks: sched/sched_wakeup, sched/sched_switch                    |
|  - Bounded BPF maps (< 8MB RAM budget)                                        |
+-------------------------------------------------------------------------------+
```

---

## 2. STRIDE Threat Assessment

| Category | Threat Description | Attack Vector | Mitigation & Technical Control | Residual Risk |
|---|---|---|---|---|
| **Spoofing** | Compromised agent impersonates healthy node. | Rogue agent sends fake telemetry with another node's ID. | Mutual TLS with node identity certificates; Node ID validation against Kubernetes node registry. | Low |
| **Tampering** | Injected telemetry values manipulate placement. | Malicious actor reports artificial 0% pressure to attract sensitive workloads. | Bidirectional clock skew validation; multi-metric cross-validation (PSI vs cgroup throttling); anomaly detection flags. | Low |
| **Repudiation** | Untraceable malicious scheduling shift. | Operator or compromised agent modifies decisions without trace. | Every placement decision emits an explainable, cryptographically signed audit log containing inputs, scores, and timestamps. | Low |
| **Information Disclosure** | Leakage of sensitive pod data via telemetry. | Workload metadata extracted from BPF ring buffer. | Telemetry only records opaque `cgroup_id` and runqueue latency microseconds; zero application payload or memory access. | Negligible |
| **Denial of Service** | Event flood exhausts scheduler memory. | Rapid context-switch cycling floods eBPF ring buffer. | Ring buffer is strictly memory-bounded ($256\text{KB}$); drops events cleanly via `bpf_ringbuf_reserve` failure handling. | Low |
| **Elevation of Privilege** | Container breakout to host kernel. | Exploit in agent container seeks root on host. | Container runs as non-root (UID 10001), drops all capabilities, adds only `CAP_BPF` + `CAP_PERFMON`; `allowPrivilegeEscalation: false`. | Negligible |
