# RFC-0005: eBPF Runqueue Latency Instrumentation

**RFC Number:** 0005  
**Title:** Low-Overhead Runqueue Latency Probing via sched_switch Tracepoints  
**Author:** ASTRA Architecture Team  
**Status:** Approved / Specification & Probes in v0.1.0  
**Target Subsystem:** `ebpf/`  

---

## 1. Problem & Motivation

User-space agents cannot measure sub-millisecond thread scheduling latency without injecting high context-switching overhead. To determine whether a node's CPU scheduler is stalling latency-sensitive tasks, ASTRA instruments the Linux kernel's task switch tracepoints.

---

## 2. eBPF Hook Design

- **Tracepoint 1:** `sched/sched_wakeup` records the enqueue timestamp $t_{\text{enqueue}}$ into a BPF hash map keyed by `pid`.
- **Tracepoint 2:** `sched/sched_switch` looks up the timestamp on task switch, computes $\Delta = t_{\text{switch}} - t_{\text{enqueue}}$, increments the log2 histogram bucket for `cgroup_id`, and cleans up the hash entry.
- **Ring Buffer:** When $\Delta > 10,000\mu\text{s}$ ($10\text{ms}$), emits a structured stall event to user space.

---

## 3. Kernel Compatibility & Privilege Matrix

| Kernel Feature | Requirement | Fallback Behavior |
|---|---|---|
| **Linux 5.8+ with BTF** | `CAP_BPF` + `CAP_PERFMON` | Primary high-resolution eBPF mode. |
| **Linux < 5.8 or No BTF** | Virtual filesystem access only | Graceful fallback to `/proc/pressure` (PSI); eBPF disabled. |
| **Ring Buffer Overflow** | Non-blocking drop | Clean drop counter increment; zero kernel hang. |

