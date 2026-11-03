# RFC-0006: Kubernetes Scheduling Framework Integration

**RFC Number:** 0006  
**Title:** Secondary Scheduler and In-Tree Plugin Architecture for Kubernetes  
**Author:** ASTRA Architecture Team  
**Status:** Approved / Specification & Manifests in v0.1.0  
**Target Subsystem:** `deploy/kubernetes`  

---

## 1. Problem & Motivation

Rather than creating a proprietary orchestrator, ASTRA integrates directly with Kubernetes using the standard **Kubernetes Scheduling Framework**.

---

## 2. Plugin Architecture

ASTRA maps its core algorithms to standard extension points:
- **`PreFilter` / `Filter`:** Enforces hard resource constraints (Allocatable CPU/Memory) and validates node readiness.
- **`PreScore` / `Score`:** Evaluates composite pressure stalls (PSI) and workload interference penalties.
- **`Reserve`:** Updates in-memory reservation states prior to binding.

---

## 3. Deployment Manifests

- Standardized manifests in [`deploy/kubernetes/`](file:///d:/astra/deploy/kubernetes/):
  - `namespace.yaml`: Hardened with `pod-security.kubernetes.io/enforce: restricted`.
  - `crd-workloadprofile.yaml`: `workloadprofiles.astra.io` CRD.
  - `rbac.yaml`: ServiceAccount, minimal Role, minimal ClusterRole (no wildcards).
  - `networkpolicy.yaml`: Default-deny ingress/egress with explicit DNS and API server rules.
  - `scheduler-deployment.yaml`: Pod Security Standards: Restricted compliant deployment.
  - `node-agent-daemonset.yaml`: Hardened DaemonSet with minimal `CAP_BPF` and `CAP_PERFMON`.

