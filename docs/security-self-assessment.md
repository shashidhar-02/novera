# ASTRA CNCF-Style Security Self-Assessment

**Standard:** CNCF Security Assurance SIG Best Practices  
**Status:** Complete  
**Date:** 2026-11-03  

---

## 1. Project Overview

ASTRA is an open-source closed-loop distributed scheduler and interference-mitigation engine informed by Linux kernel runtime telemetry (cgroups v2, PSI, and eBPF).

---

## 2. Actors & Privileges

- **Cluster Operator:** Installs ASTRA CRDs, configures scheduler deployment, manages namespaces.
- **Tenant / Application Developer:** Submits standard Kubernetes Pods or ASTRA `WorkloadProfile` CRDs. Cannot alter node scoring weights.
- **ASTRA Scheduler:** Evaluates nodes and executes `pods/binding`. Runs as non-root (UID 10001) with zero capabilities.
- **ASTRA Node Agent:** DaemonSet reading `/proc/pressure` and `/sys/fs/cgroup`. Runs as non-root (UID 10001) with `CAP_BPF` and `CAP_PERFMON`.

---

## 3. Threat Analysis & Boundaries

- **Untrusted Input Ingestion:** Workload definitions and telemetry data are treated as untrusted. Bounded by strict schemas and JSON unmarshaling fuzz tests.
- **Kernel Verifier Verification:** All eBPF probes strictly comply with the Linux kernel BPF verifier (zero unbounded loops, bounded map sizes).
- **Network Boundaries:** Default-deny NetworkPolicies isolate control-plane and agent pods from tenant workloads.

---

## 4. Vulnerability Disclosure & SSDF Alignment

- Vulnerabilities are handled under a private reporting policy ([`SECURITY.md`](../SECURITY.md)) with a 48-hour response target.
- Release artifacts are built hermetically and signed with Cosign keyless certificates in compliance with NIST SP 800-218 (SSDF).
