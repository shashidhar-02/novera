# ASTRA — Enterprise-Grade Systems Project Specification

## Autonomous Systems Telemetry & Resource Allocation

> **Project class:** Linux systems / distributed systems / cloud-native infrastructure / eBPF / Kubernetes
>
> **Target ambition:** production-grade open-source systems engineering
>
> **Primary languages:** Go, C/eBPF, Rust where justified
>
> **Primary environments:** Linux, Kubernetes, containerized workloads
>
> **Core objective:** build a measurable, explainable, fault-tolerant workload scheduling and interference-management system driven by Linux runtime telemetry.

---

# 1. Executive Summary

ASTRA is a Linux-native scheduling and resource intelligence platform that observes workload behavior at runtime and uses those observations to make better placement decisions than a scheduler relying only on static resource requests.

ASTRA is **not** a Kubernetes clone, Docker clone, dashboard, or LLM wrapper.

The project is intended to explore a serious systems question:

> Can kernel-level runtime telemetry and measured workload interference improve workload placement, utilization, and SLO adherence under realistic cluster contention?

ASTRA will begin as a deterministic simulator and Linux prototype, then progress toward Kubernetes integration, multi-node operation, fault tolerance, security hardening, reproducible benchmarks, and open-source release quality.

---

# 2. Design Principles

1. **Correctness over feature count.**
2. **Evidence over marketing.**
3. **Measured performance over assumptions.**
4. **Deterministic tests before distributed complexity.**
5. **Linux-native primitives before abstraction-heavy frameworks.**
6. **Kubernetes integration instead of Kubernetes imitation.**
7. **Least privilege by default.**
8. **Failure behavior must be explicitly defined.**
9. **Public APIs must be versioned and documented.**
10. **Every major design choice must have an RFC or design record.**

---

# 3. Problem Statement

Static scheduling commonly reasons about declared resources such as CPU and memory. Real workloads can interfere with each other through CPU contention, memory pressure, scheduler contention, I/O, networking, cache behavior, and shared kernel resources.

ASTRA models workload behavior using measured Linux telemetry and incorporates the resulting evidence into scheduling decisions.

The project must distinguish clearly between:

- raw telemetry
- correlation
- inferred interference
- experimentally validated causal relationships
- scheduling decisions

No causal claim may be made without supporting experimental methodology.

---

# 4. Scope

## 4.1 In Scope

- Linux node telemetry
- eBPF programs where technically justified
- cgroups and PSI integration
- workload identity mapping
- interference scoring
- deterministic scheduler simulator
- scheduling policy engine
- SLO-aware placement
- Kubernetes scheduling integration
- multi-node control plane
- health and failure handling
- authentication and authorization
- benchmark infrastructure
- reproducible experiments
- operator CLI
- documentation and RFCs
- CI/CD and release automation

## 4.2 Out of Scope for Initial Releases

- replacing the Linux scheduler
- replacing Kubernetes itself
- unrestricted autonomous production remediation
- unverified AI-based decisions in the scheduling critical path
- proprietary cloud lock-in
- unnecessary microservice fragmentation

---

# 5. High-Level Architecture

```text
                           +----------------------+
                           |      Operator        |
                           |   CLI / API / UI     |
                           +----------+-----------+
                                      |
                                      v
                           +----------------------+
                           |     ASTRA API        |
                           +----------+-----------+
                                      |
                 +--------------------+--------------------+
                 |                                         |
                 v                                         v
       +-------------------+                      +-------------------+
       | Control Plane     |                      | Policy Engine     |
       |                   |                      |                   |
       | scheduler         |                      | constraints       |
       | cluster state     |                      | SLO policies      |
       | placement         |                      | safety rules      |
       +---------+---------+                      +---------+---------+
                 |                                          |
                 +--------------------+---------------------+
                                      |
                                      v
                           +----------------------+
                           | Scheduling Decision  |
                           +----------+-----------+
                                      |
                   +------------------+------------------+
                   |                                     |
                   v                                     v
          +-------------------+                   +-------------------+
          | Linux Node A      |                   | Linux Node B      |
          |                   |                   |                   |
          | ASTRA agent       |                   | ASTRA agent       |
          | eBPF              |                   | eBPF              |
          | cgroups           |                   | cgroups           |
          | PSI               |                   | PSI               |
          +---------+---------+                   +---------+---------+
                    |                                       |
                    +------------------+--------------------+
                                       |
                                       v
                              +------------------+
                              | Telemetry Bus    |
                              +------------------+

Kubernetes integration:

Kubernetes Scheduler Framework / Controller
                    |
                    v
               ASTRA Policy
                    |
                    v
             ASTRA Scheduler Core
                    |
                    v
             Linux Node Telemetry
```

---

# 6. Major Components

## 6.1 Scheduler Core

Responsibilities:

- candidate generation
- hard-constraint filtering
- scoring
- placement selection
- decision explanation
- scheduler state transitions

Properties:

- deterministic under deterministic input
- bounded decision latency
- testable without Kubernetes
- no direct dependency on an LLM

## 6.2 Interference Engine

Responsibilities:

- normalize telemetry
- associate telemetry with workload identity
- compute interference features
- calculate workload-to-workload relationships
- produce confidence/quality metadata

Conceptual relation:

```text
Workload A ---- high ----> Workload B
Workload A ---- low  ----> Workload C
Workload B ---- medium --> Workload C
```

The implementation must document how each relation is obtained.

## 6.3 Node Agent

Responsibilities:

- collect kernel telemetry
- map processes/cgroups to workloads
- expose node health
- buffer or rate-limit telemetry
- detect local collection failures
- send authenticated telemetry to the control plane

The node agent must continue operating safely when the control plane is unavailable.

## 6.4 eBPF Subsystem

Potential telemetry domains:

- scheduler activity
- CPU pressure
- memory pressure
- block I/O
- networking
- process lifecycle
- cgroup behavior

Each program requires:

- attach-point rationale
- privilege requirements
- verifier/load failure behavior
- kernel compatibility policy
- cleanup behavior
- performance overhead benchmark

## 6.5 Policy Engine

Hard constraints may include:

- CPU limits
- memory limits
- affinity/anti-affinity
- node labels
- isolation constraints
- security constraints
- availability constraints

Soft objectives may include:

- SLO violation probability
- p95/p99 latency
- interference risk
- utilization
- migration cost
- energy proxy metrics when available

## 6.6 Kubernetes Integration

ASTRA must integrate with Kubernetes through supported scheduling/controller mechanisms.

The ASTRA core must remain independent from Kubernetes-specific types wherever practical.

---

# 7. Scheduling Model

Each placement decision should be explainable.

Example decision record:

```json
{
  "workload": "payments-v3",
  "candidates": ["node-a", "node-b", "node-c"],
  "rejections": {
    "node-a": ["anti-affinity"],
    "node-c": ["predicted-interference-above-threshold"]
  },
  "selected": "node-b",
  "score": 0.87,
  "dominant_factors": [
    "low-interference-risk",
    "slo-headroom",
    "sufficient-memory"
  ]
}
```

The exact schema may evolve through RFCs.

---

# 8. SLO-Aware Scheduling

Support workloads with target objectives such as:

```text
Payments: p99 < 150 ms
Search:   p99 < 400 ms
Batch:    throughput > target
```

The scheduler may optimize an aggregate objective such as:

```text
minimize:
    SLO violation risk
  + interference cost
  + migration cost
  + resource fragmentation
```

The mathematical formulation must be documented and benchmarked.

---

# 9. Standalone Simulator

The simulator is mandatory because it enables deterministic development and low-resource testing.

Workload classes:

- CPU intensive
- memory intensive
- I/O intensive
- network intensive
- latency sensitive
- bursty
- noisy neighbor

Requirements:

- deterministic seed
- configurable workload profiles
- replayable experiments
- baseline scheduler
- ASTRA scheduler
- structured output

---

# 10. Benchmark Program

Every release that changes the scheduler must be benchmarkable.

Required metrics:

- scheduling latency
- scheduling throughput
- p50 latency
- p95 latency
- p99 latency
- SLO violation rate
- CPU overhead
- memory overhead
- telemetry overhead
- migration count
- recovery time

Baseline comparison:

```text
Baseline Scheduler
        vs
ASTRA Scheduler
```

No result may be fabricated or estimated and then presented as measured.

---

# 11. Reliability Model

Explicitly define behavior for:

- node failure
- agent crash
- control-plane restart
- network partition
- stale telemetry
- duplicate telemetry
- missing telemetry
- partial cluster visibility
- invalid input
- corrupted state
- scheduler restart

Safety rule:

> stale or incomplete telemetry must never silently create an unsafe scheduling decision.

---

# 12. Security Model

Threat model must cover:

- compromised node agent
- forged telemetry
- credential theft
- malicious workload identity
- privilege escalation through eBPF
- unauthorized scheduler actions
- control-plane compromise
- malicious API clients

Requirements:

- least privilege
- authenticated transport
- authorization checks
- explicit trust boundaries
- audit logs
- secret isolation
- secure configuration defaults

---

# 13. AI Policy

AI is optional and must not be required for scheduler correctness.

Allowed uses:

- operator assistance
- anomaly explanation
- experiment summarization
- incident triage
- documentation generation

Forbidden in the critical path without explicit verification:

- unrestricted autonomous placement
- arbitrary shell execution
- arbitrary cluster mutation
- bypassing policy enforcement

All AI-originated recommendations must be treated as untrusted input.

---

# 14. Repository Layout

```text
astra/
├── cmd/
├── api/
├── scheduler/
├── interference/
├── policy/
├── controller/
├── node-agent/
├── ebpf/
├── pkg/
├── simulator/
├── benchmark/
├── integration/
├── fuzz/
├── examples/
├── deploy/
├── docs/
├── rfcs/
├── design/
├── scripts/
├── test/
├── .github/
├── README.md
├── CONTRIBUTING.md
├── SECURITY.md
├── CODE_OF_CONDUCT.md
├── LICENSE
└── CHANGELOG.md
```

The structure may evolve based on actual code boundaries.

---

# 15. RFC Governance

Create RFCs before major architectural changes.

Initial RFC set:

- RFC-0001 Architecture
- RFC-0002 Telemetry Model
- RFC-0003 Interference Model
- RFC-0004 Scheduler Design
- RFC-0005 Linux Node Agent
- RFC-0006 eBPF Subsystem
- RFC-0007 Kubernetes Integration
- RFC-0008 Security Model
- RFC-0009 Benchmark Methodology
- RFC-0010 API and Compatibility Policy

Each RFC must include:

- problem
- goals
- non-goals
- design
- alternatives
- failure modes
- security implications
- testing
- benchmark plan
- migration/compatibility impact

---

# 16. API and Compatibility

All public APIs require:

- versioning
- schema validation
- error semantics
- compatibility policy
- documentation

Never leak unstable internal implementation structures as public contracts without deliberate review.

---

# 17. Observability

ASTRA itself must expose:

- structured logs
- health endpoints
- scheduler decision metrics
- telemetry ingestion metrics
- queue depth
- stale telemetry counts
- scheduling latency
- errors
- component health

Tracing should be introduced where it provides meaningful diagnostic value.

---

# 18. Testing Strategy

## Unit

Scheduler logic, scoring, constraints, parsers, state machines.

## Integration

Control plane, node agent, telemetry pipeline, Kubernetes.

## eBPF

Program load, events, compatibility behavior, cleanup.

## Failure

Process crashes, restarts, partitions, stale data.

## Fuzzing

Input parsers, API boundaries, state transitions where useful.

## Performance

Regression tests for scheduler latency and node-agent overhead.

## End-to-End

Workload submission → placement → runtime telemetry → rescheduling/reconciliation → metrics.

---

# 19. CI/CD Quality Gates

Required checks:

- formatting
- static analysis
- unit tests
- race detector
- integration tests
- dependency/security scan
- container build
- API compatibility checks where applicable
- documentation checks

Expensive benchmark suites should run in dedicated benchmark workflows.

---

# 20. Release Engineering

Use semantic versioning where appropriate.

Release artifacts should include:

- source
- checksums
- container images
- binaries
- SBOM
- changelog
- benchmark report
- compatibility notes

A release is not production-ready merely because it compiles.

---

# 21. Enterprise-Level Non-Functional Requirements

Target, measure, and document:

- availability objectives
- recovery objectives
- horizontal scalability
- control-plane failure tolerance
- node-agent resource overhead
- telemetry backpressure
- upgrade safety
- configuration validation
- security posture
- operational diagnostics

Do not claim an SLO until it has been measured under a defined test environment.

---

# 22. Development Phases

## Phase 0 — Research and Prior Art

Document existing approaches and clearly identify ASTRA's proposed contribution.

## Phase 1 — Deterministic Simulator

Implement workloads, baseline scheduling, ASTRA scoring, and replayable experiments.

## Phase 2 — Linux Prototype

Implement minimal node agent, cgroups/PSI integration, and selected eBPF telemetry.

## Phase 3 — Real Scheduler

Connect runtime measurements to placement decisions.

## Phase 4 — Kubernetes Integration

Integrate with a real Linux Kubernetes environment.

## Phase 5 — Reliability and Security

Add failure handling, authentication, authorization, and hardening.

## Phase 6 — Benchmarks

Compare against baseline under controlled scenarios.

## Phase 7 — Open-Source Hardening

Improve APIs, documentation, CI, release automation, and contributor workflows.

---

# 23. Definition of Done

A component is complete only when:

- implementation exists
- tests exist
- failure behavior is defined
- security implications are documented
- public interfaces are documented
- observability exists where useful
- performance impact is measured where relevant
- CI passes
- limitations are documented

---

# 24. Project Success Criteria

ASTRA is successful when it can demonstrate, with reproducible experiments, that:

1. it measures real Linux runtime behavior;
2. it can associate measurements with workloads;
3. it can detect meaningful interference patterns;
4. those patterns influence scheduling decisions;
5. the resulting decisions are explainable;
6. the overhead of the telemetry path is measured;
7. the scheduler survives defined failure scenarios;
8. Kubernetes integration works on a real Linux cluster;
9. benchmark methodology is reproducible;
10. the repository is maintainable by engineers other than the original author.

---

# 25. Positioning

ASTRA should be presented as:

> A research-driven open-source systems project exploring Linux-aware, eBPF-assisted, SLO-driven workload scheduling and interference management.

Do not present it as a Kubernetes replacement or as proven superior to Kubernetes until evidence exists.

---

# 26. Final Standard

The project should feel like the beginning of a real infrastructure codebase:

- small core
- deep engineering
- explicit interfaces
- serious testing
- measurable experiments
- reproducible builds
- security boundaries
- operational documentation
- open-source contribution standards

**Ambition:** Kubernetes/Linux-level engineering discipline.

**Requirement:** earn every claim through implementation and evidence.
