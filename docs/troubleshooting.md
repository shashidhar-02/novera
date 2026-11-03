# ASTRA Troubleshooting & Triage Guide

**Document ID:** ASTRA-TRIAGE-001  
**Scope:** Triage and failure remediation  

---

## 1. Common Operational Scenarios

### Scenario 1: Workload Placement Marked `TelemetryDegraded`
- **Symptom:** `astra decision explain` outputs `telemetry stale, missing, or clock-skewed: pressure score set to neutral default (50.0)`.
- **Root Cause:**
  1. Node agent has crashed or is partitioned.
  2. Host clock skew: System clock on the node differs from control plane by $> 5\text{s}$.
- **Resolution:**
  1. Inspect agent logs: `kubectl logs -n astra-system daemonset/astra-node-agent`.
  2. Verify NTP synchronization across cluster nodes: `chronyc tracking` or `timedatectl`.

---

### Scenario 2: Scheduler Reports `ErrNoFitNodes`
- **Symptom:** Pod remains in `Pending` state with event `no candidate node satisfies hard placement constraints`.
- **Root Cause:** All available nodes lack sufficient unreserved CPU (`cpu_request_millis`) or Memory (`memory_request_bytes`).
- **Resolution:**
  1. Inspect allocatable capacity: `astra node status`.
  2. Adjust workload requests or scale cluster nodes.

---

### Scenario 3: eBPF Probe Fails to Attach
- **Symptom:** Agent logs `failed to attach sched:sched_switch tracepoint: operation not permitted`.
- **Root Cause:** Node kernel lacks `CONFIG_BPF_EVENTS=y` or container is missing `CAP_BPF` / `CAP_PERFMON`.
- **Resolution:** The agent automatically falls back to `/proc/pressure` (PSI) mode. No operator intervention required unless eBPF runqueue histograms are mandatory.
