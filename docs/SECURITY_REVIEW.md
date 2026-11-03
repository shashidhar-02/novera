# ASTRA Formal Security Review

**Review Standards:** Linux Kernel Security Guidelines, Kubernetes Security Guidance, NIST SP 800-218 (SSDF), SLSA v1.2  
**Date:** 2026-11-03  
**Status:** Passed with Documented Boundary Controls  

---

## 1. Linux Kernel & OS Security

### 1.1 Process Isolation & Capabilities
- **Non-Root Execution:** All user-space processes (Scheduler and Node Agent) execute under unprivileged non-root accounts (UID `10001`, GID `10001`).
- **Capability Bounding:**
  - Scheduler container: Drops `ALL` capabilities (`securityContext.capabilities.drop: ["ALL"]`).
  - Node Agent container: Drops `ALL` capabilities, adding only `BPF` and `PERFMON` (Linux 5.8+). Full `CAP_SYS_ADMIN` is explicitly rejected.
- **Seccomp Filters:** Configured with `seccompProfile.type: RuntimeDefault`, restricting dangerous or legacy syscalls (`ptrace`, `reboot`, `kexec_load`).

### 1.2 Virtual Filesystem Protections
- Host path mounts to `/sys/fs/cgroup` and `/proc/pressure` are mounted strictly with `readOnly: true`.
- Zero write access granted to any host filesystem path.

---

## 2. eBPF Program Security & Safety

- **Program Type:** `BPF_PROG_TYPE_TRACEPOINT` attaching to `sched:sched_wakeup` and `sched:sched_switch`.
- **Verifier Compliance:**
  - All loops bounded and unrolled with `#pragma unroll`.
  - Memory access verified via `bpf_probe_read_kernel_str`.
  - Zero pointer arithmetic on unchecked kernel addresses.
- **Resource Limits:**
  - Total in-kernel map allocation strictly bounded to $< 8\text{ MB}$.
  - Ring buffer sized to $256\text{ KB}$ with lockless, non-blocking reservation.

---

## 3. Kubernetes Security & Workload Hardening

### 3.1 Pod Security Standards
- Namespace [`astra-system`](../deploy/kubernetes/namespace.yaml) is labeled with:
  - `pod-security.kubernetes.io/enforce: restricted`
  - `pod-security.kubernetes.io/audit: restricted`
  - `pod-security.kubernetes.io/warn: restricted`
- Workload manifests comply with the **Restricted** profile:
  - `runAsNonRoot: true`
  - `allowPrivilegeEscalation: false`
  - `readOnlyRootFilesystem: true`
  - `capabilities.drop: ["ALL"]`

### 3.2 RBAC Least-Privilege Matrix

| ServiceAccount | Scope | Target Resource | Allowed Verbs | Explicit Justification |
|---|---|---|---|---|
| `astra-scheduler` | Cluster | `nodes` | `get`, `list`, `watch` | Required to discover node allocatable capacity and topology labels. |
| `astra-scheduler` | Cluster | `pods` | `get`, `list`, `watch` | Required to track cluster workload allocations. |
| `astra-scheduler` | Cluster | `pods/binding` | `create` | Required to bind scheduled pods to selected nodes. |
| `astra-scheduler` | Cluster | `workloadprofiles.astra.io` | `get`, `list`, `watch` | Required to read SLO targets and workload classes. |
| `astra-scheduler` | Namespace (`astra-system`) | `coordination.k8s.io/leases` | `get`, `create`, `update` | Required for multi-replica leader election. |

*Zero wildcard (`*`) resources or verbs exist in any RBAC manifest.*

### 3.3 Network Isolation (NetworkPolicy)
- [`deploy/kubernetes/networkpolicy.yaml`](../deploy/kubernetes/networkpolicy.yaml) establishes a **default-deny-all** policy for both ingress and egress in `astra-system`.
- Explicit whitelisted egress to `kube-dns` (port 53 UDP/TCP) and Kubernetes API server (ports 443/6443).

---

## 4. Software Supply Chain & Vulnerability Posture

- **Zero Third-Party Dependencies:** Core scheduler and telemetry collectors rely exclusively on the Go standard library. There are **zero transitive dependency vulnerabilities** or CVE risks.
- **Hermetic Build Reproducibility:** Deterministic Go builds with pinned compiler version (`go1.27.3`).
- **Signing & Provenance:** GitHub Actions CI includes Cosign image signing and SLSA Level 2 provenance generation steps.
