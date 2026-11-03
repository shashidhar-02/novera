# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

---

## Threat Model Overview

ASTRA is designed with a defense-in-depth posture:

1. **Kernel Telemetry Boundaries:**
   - eBPF programs run sandboxed inside the Linux kernel and are validated by the kernel eBPF verifier.
   - Node agents require only `CAP_BPF` and `CAP_PERFMON` (or unprivileged read access to `/proc/pressure` and `/sys/fs/cgroup`). Full `CAP_SYS_ADMIN` is explicitly rejected.
2. **Control-Plane Hardening:**
   - Communication between the node agent and control plane utilizes mutual TLS (mTLS).
   - Node telemetry is bounded and validated before ingestion; out-of-range values are discarded and flagged.
   - Stale telemetry (>5s) triggers an automatic fallback to static capacity scheduling, preventing malicious or delayed telemetry from inducing denial of service.
3. **No Arbitrary Execution:**
   - The scheduler never executes arbitrary shell commands or user-supplied scripts based on telemetry or model output.

---

## Reporting a Vulnerability

If you discover a security vulnerability in ASTRA, please report it privately:

- **Email:** `security@astra-orchestration.org` (or open a confidential GitHub Advisory)
- **Response Timeline:** We aim to acknowledge receipt within 48 hours and provide an initial assessment within 5 business days.
- Please do **not** open a public issue before the vulnerability has been triaged and patched.
