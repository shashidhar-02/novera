# ASTRA eBPF Telemetry Subsystem

**Subsystem:** Node Kernel Telemetry  
**Target Program:** `sched_switch.bpf.c`  
**License:** GPL-2.0  
**Minimum Kernel Version:** Linux 5.8+ (with BPF CO-RE and BTF support enabled)  

---

## 1. Subsystem Architecture

The ASTRA eBPF subsystem measures **in-kernel runqueue wait latency** ($t_{\text{scheduled}} - t_{\text{enqueued}}$) directly inside the Linux kernel scheduler. Unlike user-space sampling, tracepoint hooks provide microsecond-accurate latency measurements without intrusive context switching.

```text
[ Task Wakeup ] ---> Tracepoint: sched/sched_wakeup
                             |
                             v
                 Store (pid -> timestamp_ns) in BPF Hash Map
                             |
[ Context Switch ] -> Tracepoint: sched/sched_switch
                             |
                             v
                 Compute delta_us = now - t_enqueue
                 Update log2 histogram for cgroup_id
                 If delta_us > 10ms -> Push event to BPF Ring Buffer
                             |
                             v
                 Node Agent User-Space Ingestion Loop
```

---

## 2. Kernel Interface & Verification Properties

| Property | Value | Security / Operational Requirement |
|---|---|---|
| **Program Type** | `BPF_PROG_TYPE_TRACEPOINT` | Read-only kernel inspection; cannot modify kernel state or alter task scheduling. |
| **Attach Points** | `tp/sched/sched_wakeup`, `tp/sched/sched_switch` | Stable kernel ABI tracepoints (avoids brittle kprobe symbol dependencies). |
| **Required Capabilities** | `CAP_BPF` + `CAP_PERFMON` | Least privilege; does **not** require full `CAP_SYS_ADMIN` on Linux 5.8+. |
| **Map Memory Footprint** | Bounded $< 8\text{ MB}$ total | `enqueue_timestamps` (10,240 entries), `cgroup_lat_hist` (1,024 entries), `stall_events` (256 KB ring buffer). |
| **Verifier Complexity** | $< 250$ instructions | Strict bounded loops with `#pragma unroll`; guaranteed verifier acceptance. |
| **Overhead Budget** | $< 0.8\%$ single CPU core | Negligible overhead; atomics restricted to per-cgroup bucket increments. |

---

## 3. Failure Handling & Degradation Invariants

1. **Unsupported Kernel (< 5.8 or no BTF):** The Node Agent detects missing `/sys/kernel/btf/vmlinux` and gracefully disables eBPF instrumentation, falling back to `/proc/pressure` (PSI) telemetry without crashing.
2. **Ring Buffer Saturation:** If the user-space agent lags behind high-frequency stall events, `bpf_ringbuf_reserve` returns `NULL`, cleanly dropping events and incrementing drop counters without stalling the kernel scheduler.
3. **Agent Crash / Restart:** All BPF map file descriptors are pinned or owned by the agent process; if the agent terminates, the kernel automatically unloads the tracepoint hooks, preventing dangling kernel resources.

