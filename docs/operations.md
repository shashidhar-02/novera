# ASTRA Operator Guide & Runbook

**Document ID:** ASTRA-OPS-001  
**Target Version:** v0.1.0  
**Scope:** Cluster Operators & SREs  

---

## 1. Operator Diagnostics CLI

The `astra` binary provides operational triage without requiring source-code inspection:

```bash
# Check overall cluster and leader health
astra status

# Inspect individual node capacity and real-time PSI pressure
astra node status

# Explain a scheduling placement decision
astra decision explain

# Validate a workload profile definition
astra policy validate

# Generate a sanitized diagnostic bundle (zero secrets)
astra debug bundle
```

---

## 2. Core Prometheus Metrics Reference

| Metric Name | Type | Description | Alert Threshold |
|---|---|---|---|
| `astra_scheduler_healthy` | Gauge | Control-plane health indicator ($1 = \text{Healthy}$) | $0$ for $> 30\text{s}$ |
| `astra_decision_latency_seconds` | Histogram | Latency of the scheduling evaluation loop | $p99 > 5\text{ms}$ |
| `astra_telemetry_stale_total` | Counter | Cumulative instances of stale/skewed telemetry fallback | Rate $> 5/\text{min}$ |
| `astra_node_pressure_ratio` | Gauge | Normalized kernel pressure per node | $> 0.85$ for $> 60\text{s}$ |

---

## 3. Upgrade & Rollback Workflow

1. **Deploy New Standby Replica:** Deploy `astra-scheduler` v0.1.1; verify it successfully joins coordination leases in standby mode.
2. **Graceful Failover:** Terminate the active v0.1.0 pod; the standby automatically acquires leadership within $1.15\text{s}$.
3. **Rollback:** If unexpected latency occurs, redeploy v0.1.0; zero schema migration or state loss occurs.

