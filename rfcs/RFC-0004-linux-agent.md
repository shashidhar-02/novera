# RFC-0004: Linux Node Agent Architecture

**RFC Number:** 0004  
**Title:** Linux cgroups v2, PSI Ingestion, and Least-Privilege Daemon Architecture  
**Author:** ASTRA Architecture Team  
**Status:** Approved / Implemented in v0.1.0  
**Target Subsystem:** `pkg/agent`  

---

## 1. Problem & Motivation

Distributed schedulers need high-frequency ($1\text{Hz}$) node health measurements without running heavy user-space monitoring agents (e.g. Prometheus Node Exporter) that consume significant CPU/memory or demand root privileges.

ASTRA's Node Agent reads kernel state directly from `/proc/pressure` and `/sys/fs/cgroup` with sub-millisecond execution overhead and minimal memory footprints ($< 16\text{MB}$).

---

## 2. Ingestion Primitives

1. **Pressure Stall Information (PSI):**
   Reads `/proc/pressure/cpu`, `/proc/pressure/memory`, `/proc/pressure/io`.
   Extracts `some avg10`, `some avg60`, `full avg10`, `full avg60`.
2. **cgroups v2 Throttling:**
   Reads `/sys/fs/cgroup/.../cpu.stat`.
   Extracts `nr_periods`, `nr_throttled`, and computes $\text{ThrottledRatio} = \frac{\text{nr\_throttled}}{\text{nr\_periods}}$.
3. **Telemetry Packaging:**
   Packages readings into `model.NodeTelemetry` with UTC timestamp and monotonically increasing sequence numbers.

---

## 3. Security & Isolation

- **Non-Root Execution:** Runs as UID `10001` (`astra-agent`).
- **Read-Only Virtual FS:** Mounts `/sys/fs/cgroup` and `/proc/pressure` with `readOnly: true`.
- **Capability Bounding:** Drops all Linux capabilities (`CAP_SYS_ADMIN` rejected).
