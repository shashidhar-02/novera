//go:build ignore
// +build ignore
// SPDX-License-Identifier: GPL-2.0
// Copyright (c) 2026 ASTRA Authors

#include <linux/types.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

char LICENSE[] SEC("license") = "GPL";

#define MAX_ENTRIES 10240
#define HIST_SLOTS  16

// Event emitted to userspace when a severe runqueue stall occurs (> 10,000 us).
struct runq_stall_event {
    __u64 cgroup_id;
    __u64 latency_us;
    __u32 pid;
    char  comm[16];
};

// Map storing wakeup timestamp per process: pid -> timestamp_ns
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, MAX_ENTRIES);
    __type(key, __u32);
    __type(value, __u64);
} enqueue_timestamps SEC(".maps");

// Map storing log2 latency histogram per cgroup: cgroup_id -> histogram buckets
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u64);
    __type(value, __u64[HIST_SLOTS]);
} cgroup_lat_hist SEC(".maps");

// BPF ring buffer for streaming latency anomaly events to user-space agent
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256 KB ring buffer
} stall_events SEC(".maps");

// Tracepoint: sched/sched_wakeup
// Records the timestamp when a task is enqueued onto the CPU runqueue.
SEC("tp/sched/sched_wakeup")
int handle_sched_wakeup(void *ctx)
{
    struct {
        __u64 pad;
        char  comm[16];
        __u32 pid;
        __u32 prio;
        __u32 success;
        __u32 target_cpu;
    } *args = ctx;

    __u32 pid = args->pid;
    __u64 ts = bpf_ktime_get_ns();

    bpf_map_update_elem(&enqueue_timestamps, &pid, &ts, BPF_ANY);
    return 0;
}

// Tracepoint: sched/sched_switch
// Measures (t_scheduled - t_enqueued) for the incoming task.
SEC("tp/sched/sched_switch")
int handle_sched_switch(void *ctx)
{
    struct {
        __u64 pad;
        char  prev_comm[16];
        __u32 prev_pid;
        __u32 prev_prio;
        __u64 prev_state;
        char  next_comm[16];
        __u32 next_pid;
        __u32 next_prio;
    } *args = ctx;

    __u32 next_pid = args->next_pid;
    __u64 *tsp = bpf_map_lookup_elem(&enqueue_timestamps, &next_pid);
    if (!tsp) {
        return 0; // Task was not tracked on wakeup (e.g. initial boot task)
    }

    __u64 now = bpf_ktime_get_ns();
    __u64 delta_ns = now - *tsp;
    bpf_map_delete_elem(&enqueue_timestamps, &next_pid);

    __u64 delta_us = delta_ns / 1000;
    __u64 cgroup_id = bpf_get_current_cgroup_id();

    // Calculate log2 bucket index
    __u32 slot = 0;
    __u64 val = delta_us;
    #pragma unroll
    for (int i = 0; i < HIST_SLOTS - 1; i++) {
        if (val > 1) {
            slot++;
            val >>= 1;
        }
    }
    if (slot >= HIST_SLOTS) {
        slot = HIST_SLOTS - 1;
    }

    __u64 *hist = bpf_map_lookup_elem(&cgroup_lat_hist, &cgroup_id);
    if (hist) {
        __sync_fetch_and_add(&hist[slot], 1);
    }

    // Check for severe runqueue latency stall (> 10 ms)
    if (delta_us > 10000) {
        struct runq_stall_event *evt = bpf_ringbuf_reserve(&stall_events, sizeof(*evt), 0);
        if (evt) {
            evt->cgroup_id = cgroup_id;
            evt->latency_us = delta_us;
            evt->pid = next_pid;
            bpf_probe_read_kernel_str(evt->comm, sizeof(evt->comm), args->next_comm);
            bpf_ringbuf_submit(evt, 0);
        }
    }

    return 0;
}

