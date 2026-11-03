# RFC-0008: API Versioning & Evolution Policy

**RFC Number:** 0008  
**Title:** Backward-Compatible Evolution, Deprecation Lifecycle, and Schema Governance  
**Author:** ASTRA Architecture Team  
**Status:** Approved / v0.1.0  
**Target Subsystem:** `api/` & `pkg/model`  

---

## 1. Scope & Semantic Versioning

ASTRA adheres strictly to **Semantic Versioning 2.0.0** (`MAJOR.MINOR.PATCH`):
- `v1alpha1`: Experimental / Pre-production schemas (subject to change with deprecation notices).
- `v1beta1`: Feature-complete, backward-compatible schemas within a major version.
- `v1`: Long-term stable production APIs with a minimum 12-month deprecation window.

---

## 2. Invariants

1. **No Breaking Field Removals:** Fields in public schemas (`model.Workload`, `model.NodeTelemetry`) may not be deleted or have their types modified within a minor version.
2. **Graceful Defaulting:** Any new field added to telemetry or workload specs must be optional with deterministic zero-value fallbacks.
3. **Explicit Validation:** All public APIs must implement `.Validate()` returning clear errors before internal processing.
