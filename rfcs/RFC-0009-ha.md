# RFC-0009: High Availability & Lease-Based Leader Election

**RFC Number:** 0009  
**Title:** Active-Standby Control Plane Coordination via Kubernetes Leases  
**Author:** ASTRA Architecture Team  
**Status:** Approved / Implemented in `pkg/coordination`  
**Target Subsystem:** `pkg/coordination`  

---

## 1. Problem & Motivation

In high-density cloud-native clusters, the scheduling control plane cannot be a single point of failure. Running multiple scheduler replicas concurrently requires distributed mutual exclusion to prevent split-brain dual-binding races where two schedulers assign different workloads to the same stranded capacity simultaneously.

ASTRA implements a lease-based active-standby leader election protocol modeled after `coordination.k8s.io/leases`.

---

## 2. Protocol Specification

### 2.1 State Model
```text
[ Standby Candidate ]
       |
       | Periodic Poll (RenewInterval = 3s)
       v
Is Lease Expired or Missing?
       |--- YES ---> Attempt Atomic CAS Acquire
       |                    |
       |                    +--- SUCCESS ---> [ Active Leader ] (OnStartedLeading)
       |                    |
       |                    +--- CONFLICT --> Remain Standby
       |
       +--- NO ----> Check Current Holder Identity
                            |
                            +--- Self Holder --> Renew Lease Timestamp
                            |
                            +--- Other Holder -> Track Leader Transitions
```

### 2.2 Time Invariants
- `LeaseDurationSeconds`: $15\text{s}$ (Standard Kubernetes default).
- `RenewInterval`: $3\text{s}$ (Heartbeat cadence; $5\times$ smaller than duration to withstand transient network partitions).
- **Lease Expiry Definition:** A lease is valid if $\text{Now} \le \text{RenewTime} + \text{LeaseDurationSeconds}$.

### 2.3 Failover & State Continuity
When the active leader crashes or loses network connectivity:
1. Heartbeats cease; after $15\text{s}$, `IsExpired` returns `true`.
2. Competing standbys attempt atomic acquisition.
3. The winner increments `LeaderTransitions` and invokes `OnStartedLeading(ctx)`.
4. The former leader, upon detecting context cancellation or lease loss, immediately releases control via `OnStoppedLeading()`.

---

## 3. Testing & Verification

Proven in [`pkg/coordination/lease_test.go`](../pkg/coordination/lease_test.go):
- Single-node acquisition and periodic renewal.
- Automatic failover within $1.15\text{s}$ upon simulated primary crash.
- 5-worker concurrent contention with strict single-leader mutual exclusion.

