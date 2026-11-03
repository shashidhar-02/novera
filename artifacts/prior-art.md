# ASTRA Prior Art & Comparative Systems Analysis

**Document ID:** ASTRA-ART-001  
**Date:** 2026-11-03  
**Status:** Approved / Technical Baseline  
**Scope:** Literature review, systems research analysis, and cloud-native scheduler comparison for ASTRA.

---

## 1. Overview & Research Context

Modern distributed cluster schedulers typically rely on **static, declared resource specifications** (`requests` and `limits` in Kubernetes, or quota reservations in Borg/Mesos). While this abstraction decouples application developers from bare-metal details, it introduces the **Noisy-Neighbor Problem** and **Stranded Resource Fragility**:

1. **Micro-architectural Contention:** Workloads sharing a CPU socket contend for L3 cache (LLC), memory bus bandwidth, CPU execution pipelines, and NUMA memory interconnects. Declared vCPU/Memory numbers are blind to this contention.
2. **Kernel-level Queue Stalling:** Runqueue latency (`runqlat`), I/O elevator stalls, and cgroup throttling (`cpu.stat:nr_throttled`) degrade tail latency ($p99$ and $p99.9$) by orders of magnitude even when average CPU utilization appears modest (<50%).
3. **Static Allocation Inefficiencies:** Operators over-provision requests by 200–500% to guarantee latency targets, leading to massive infrastructure waste.

ASTRA addresses this by building a feedback-driven scheduler informed by kernel-level runtime telemetry (cgroups v2, Linux Pressure Stall Information, and low-overhead eBPF tracepoints).

Below is the structured analysis of prior art, establishing exactly what ASTRA reuses, what it diverges from, and what constitutes its distinct systems contribution.

---

## 2. Comparative Matrix

| System / Paper | Domain | Resource Model | Contention Awareness | Kernel Telemetry | ASTRA Relationship |
|---|---|---|---|---|---|
| **K8s Default Scheduler (`kube-scheduler`)** | Container Orchestration | Static `requests` & `limits`, topology labels | None (purely capacity & affinity based) | None | **Integrates / Replaces Scoring phase** via Scheduling Framework plugin. |
| **K8s Trimaran (Load-Watcher / TargetLoadPacking)** | K8s Scheduling SIG | Dynamic average CPU/Memory metrics | Low (reacts to coarse Prometheus/metrics-server utilization) | None (no micro-architectural or PSI telemetry) | **Diverges:** Trimaran uses time-lagged coarse averages. ASTRA uses sub-second kernel PSI and runqueue distribution. |
| **Volcano / Apache YuniKorn** | Batch / HPC / AI Scheduling | Gang scheduling, queue hierarchies, static limits | None (optimizes throughput & fairness, not latency SLOs) | None | **Diverges:** Focused on job queuing, not kernel-level interference mitigation. |
| **Google Borg (Resource Estimation & FSD)** | Proprietary Orchestration | Machine-learned usage estimation + batch reclamation | Medium (evaluates CPU throttling, out-of-memory events) | Proprietary node profiling (Cadvisor/Borglet internals) | **Reuses concept:** Dynamic capacity estimation; **Diverges:** Open-source, eBPF/PSI-driven, SLO-causal. |
| **Bubble-Up (Mars et al., ISCA 2011)** | Academic Systems Research | Micro-architectural sensitivity profiling | High (measures LLC/Memory bus pressure via synthetic probes) | Hardware performance counters (PMUs) | **Inspiration:** Workload interference classification; **Changes:** Bubble-Up required destructive pre-deployment testing. ASTRA monitors passive runtime metrics. |
| **Caladan (Fried et al., OSDI 2020)** | OS / Fast CPU Allocation | 100-microsecond core allocation & memory bandwidth control | High (allocates CPU cores to latency-critical tasks) | Hardware PMUs, memory bus counters | **Inspiration:** Microsecond interference reaction; **Diverges:** Caladan is single-node OS scheduling. ASTRA is cluster-level placement. |
| **Cilium / Tetragon** | eBPF Networking & Security | Network endpoints, kernel syscall tracing | N/A (Networking / Security focus) | eBPF kprobes, tracepoints, perf rings | **Reuses:** eBPF loading mechanisms, CO-RE standards, ringbuffer patterns. |

---

## 3. Deep-Dive Systems Analysis

### 3.1 Kubernetes Default Scheduler (`kube-scheduler`)

- **Existing Solution:** Uses a two-phase `Filter` (predicates) and `Score` (priorities) pipeline. Filters eliminate nodes lacking available unreserved request capacity. Scorers evaluate node affinity, spread constraints, and balanced resource allocation (`NodeResourcesBalancedAllocation`).
- **What ASTRA Reuses:**
  - The plugin-based lifecycle interfaces from the upstream Kubernetes Scheduling Framework (`PreFilter`, `Filter`, `PostFilter`, `PreScore`, `Score`, `Reserve`, `Permit`).
  - Strict preservation of hard isolation constraints (taints, tolerations, affinity, volume bindings).
- **What ASTRA Changes:**
  - Replaces or augments the default `Score` phase with an **Interference & Pressure Scorer**.
  - Replaces blind static capacity assumptions with an effective capacity calculated from kernel-level pressure indicators.
- **ASTRA's Distinct Contribution:**
  - Injects a continuous feedback loop: actual kernel stall telemetry from node agents modifies node scoring weights for incoming workloads in real time.

### 3.2 Linux Cgroups v2 & Pressure Stall Information (PSI)

- **Existing Solution:** 
  - **cgroups v2:** Provides hierarchical resource distribution (`cpu.max`, `cpu.weight`, `memory.high`, `memory.max`, `io.weight`). Exposes per-cgroup throttling statistics (`cpu.stat: nr_throttled, throttled_usec`).
  - **PSI (`/proc/pressure/{cpu,memory,io}`):** Quantifies percentage of time tasks are stalled waiting for CPU runqueues, memory page reclaim/refault, and blocked I/O. Distinguishes `some` (at least one task stalled) from `full` (all tasks stalled, wasted CPU cycles).
- **What ASTRA Reuses:**
  - Uses PSI and `cpu.stat` as primary, zero-overhead kernel ground truth. Reading `/proc/pressure` requires zero kernel patching and incurs sub-microsecond overhead.
- **What ASTRA Changes:**
  - Traditional systems use PSI for reactive local eviction (e.g., `systemd-oomd`, Kubernetes eviction manager). ASTRA exports PSI derivatives as **cluster-wide scheduling telemetry** to prevent placing workloads on nodes teetering on saturation cliffs.
- **ASTRA's Distinct Contribution:**
  - Multi-dimensional pressure projection: Combines `cpu:some`, `memory:full`, and `io:some` into a unified Workload Stress Tensor, determining not just if a node is busy, but *which subsystem is saturated*.

### 3.3 eBPF Subsystem & Runtime Kernel Probes

- **Existing Solution:** eBPF permits running sandboxed, verifier-guaranteed bytecode inside the Linux kernel on kprobes, tracepoints, and perf events. Cilium, Falco, and BPFdoor have proven its production viability for networking and security.
- **What ASTRA Reuses:**
  - BPF CO-RE (Compile Once – Run Everywhere) using `vmlinux.h` and BTF (BPF Type Format).
  - High-throughput `BPF_MAP_TYPE_RINGBUF` for lockless kernel-to-userspace telemetry streaming.
  - Safe attach points on stable tracepoints: `sched:sched_switch`, `sched:sched_wakeup_new`, and `block:block_rq_issue`.
- **What ASTRA Avoids:**
  - **No kprobe abuse:** Avoids unstable kprobe hooks on non-exported kernel symbols that break across kernel point releases.
  - **No high-frequency lock contention:** Does not track every individual instruction or cache miss at millions of events/sec in software. Uses hardware sampling buffers or aggregation maps inside BPF programs.
- **ASTRA's Distinct Contribution:**
  - Workload Cgroup Runqueue Histogram: Instead of raw event logging, the eBPF program aggregates runqueue latency ($t_{\text{scheduled}} - t_{\text{enqueued}}$) inside a BPF hash map keyed by `cgroup_id`. Only the histogram buckets or percentile summaries are read by the userspace node agent.

### 3.4 Workload Interference Modeling (Academic & Industry Precedents)

- **Existing Solution:** Academic systems (Bubble-Up, Paragon, Quasar, Caladan) demonstrated that colocating latency-sensitive (LS) workloads with batch/best-effort (BE) workloads causes non-linear latency degradation. However, they either:
  1. Required intrusive synthetic stress probes before production admission (Bubble-Up).
  2. Required specialized OS kernels or direct hardware PMU access that violates container abstraction (Caladan).
- **What ASTRA Reuses:**
  - Classification of workloads into sensitivity profiles:
    - **Latency-Sensitive (LS):** Strict $p99$ tail SLOs, low tolerance for CPU scheduling delays.
    - **Throughput-Oriented / Batch (TH):** CPU-bound or memory-bound, latency-insensitive.
    - **I/O-Intensive (IO):** Heavy disk or network read/write patterns.
- **What ASTRA Changes:**
  - **Passive Non-Intrusive Learning:** ASTRA observes co-scheduled performance passively without executing intrusive microbenchmarks against live user containers.
  - **Explicit Non-Causal Honesty:** ASTRA distinguishes between *observed correlation* (workload A and B co-run while node PSI spiked) and *validated causation* (isolating controlled variable shifts). It never claims causal inference without active intervention tests.
- **ASTRA's Distinct Contribution:**
  - **Interference Affinity Graph:** Maintains a dynamic compatibility matrix between workload classes. When workload $W_1$ (e.g., Lucene full-text search) consistently experiences runqueue spikes when co-located with $W_2$ (e.g., RocksDB compaction), their co-scheduling affinity score drops, steering the scheduler toward orthogonal placement.

---

## 4. Summary of ASTRA's Architectural Contributions

1. **A Unified Kernel-Telemetry Feedback Loop:** Connecting Linux cgroups v2, PSI, and eBPF runqueue telemetry directly to distributed scheduling placement decisions.
2. **SLO-Aware Multi-Objective Scoring:** Mathematical scoring that minimizes predicted $p99$ tail latency violations while optimizing cluster resource compaction.
3. **Graceful Fallback & Deterministic Safety Invariants:** Formal policy boundaries ensuring that stale, missing, or compromised agent telemetry falls back gracefully to standard capacity constraints without causing cascading cluster failures.
4. **Standalone Portability:** A completely decoupled core engine capable of running in simulation mode, Linux standalone prototype mode, or full Kubernetes integration.

