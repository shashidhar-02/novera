# ASTRA System Compatibility Matrix

**Document ID:** ASTRA-COMPAT-001  
**Status:** Approved  
**Target Release:** v0.1.0  

---

## 1. Operating System & Kernel Compatibility

| Environment | Kernel Version | Telemetry Capabilities | Status | Notes |
|---|---|---|---|---|
| **Linux (Modern)** | 5.8+ (cgroups v2 + BTF) | Full PSI + cgroups v2 + eBPF tracepoints | **SUPPORTED** | Primary production deployment target. |
| **Linux (Legacy)** | 4.20 - 5.7 | cgroups v2 + PSI (eBPF requires CAP_SYS_ADMIN) | **DEGRADED** | Fallback to PSI filesystem parsing; eBPF disabled. |
| **Linux (Pre-PSI)**| < 4.20 | cgroups v1 only (No PSI) | **UNSUPPORTED** | Lacks kernel pressure stall instrumentation. |
| **Windows / macOS**| N/A | Pure user-space simulation | **DEVELOPMENT ONLY**| Simulator CLI & dual-engine verification runner. |

---

## 2. Kubernetes & Container Runtimes

| Component | Supported Versions | Verification Status |
|---|---|---|
| **Kubernetes Control Plane** | v1.28, v1.29, v1.30, v1.31 | Manifests adhere to Pod Security Standards (Restricted) |
| **Container Runtimes** | containerd 1.6+, CRI-O 1.26+ | Required for cgroups v2 hierarchy support |
| **CNI Plugins** | Calico, Cilium, Flannel | NetworkPolicies require CNI with enforcement capabilities |

---

## 3. Toolchains

- **Go:** 1.22, 1.23, 1.24 (Tested with `go1.27.3 windows/amd64` and Linux CI).
- **Python:** 3.10, 3.11, 3.12, 3.13, 3.14 (Verified with `Python 3.14.6`).
