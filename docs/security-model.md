# ASTRA Security Model & Principles

**Document ID:** ASTRA-SEC-001  
**Status:** Approved  

---

## 1. Principles of Security

1. **Least Privilege by Default:** No component receives root access or broad capabilities unless technically unavoidable.
2. **Hardened Workload Profiles:** All deployments adhere to **Kubernetes Pod Security Standards: Restricted Profile**.
3. **Defensive Telemetry Ingestion:** All telemetry streams are strictly bounded, validated for clock-skew, and checked for numerical anomalies before entering the scheduling pipeline.
4. **Zero-Trust Network Isolation:** Deployments enforce default-deny NetworkPolicies for both ingress and egress.
5. **Supply-Chain Integrity:** Release artifacts are reproducibly built, signed with Sigstore/Cosign, and accompanied by SPDX/CycloneDX SBOMs.

---

## 2. Privileged Subsystems Justification

| Component | Privilege Requested | Technical Justification | Impact of Compromise | Mitigation |
|---|---|---|---|---|
| **Node Agent** | `hostPID: true` | Required to resolve host container processes to cgroups v2 paths. | Read-only process table visibility. | Non-root execution (UID 10001); read-only mounts; capabilities restricted to `BPF` and `PERFMON`. |
| **Node Agent** | `CAP_BPF` + `CAP_PERFMON` | Required to load eBPF tracepoint programs on Linux 5.8+. | Ability to attach tracing probes. | Verifier enforces bounded execution; maps bounded to $< 8\text{MB}$; read-only tracepoints. |
