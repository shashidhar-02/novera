# Contributing to ASTRA

Thank you for your interest in contributing to ASTRA. As an infrastructure-grade distributed systems project, ASTRA upholds rigorous software engineering, architectural discipline, and testing standards.

---

## 1. Principles of Contribution

1. **RFC First for Architectural Changes:** Any modification to public APIs, core scoring mathematics, scheduling invariants, or telemetry schemas must begin with an RFC in the `rfcs/` directory.
2. **Correctness Over Speed:** High test coverage, deterministic tests, and edge-case validation are prerequisites for code review.
3. **Reproducible Benchmarks:** Any performance improvement claim must include automated benchmarks and reproducibility steps. Never submit unverified performance figures.
4. **Least Privilege & Security:** Never introduce dependencies or kernel probes that violate the principle of least privilege.

---

## 2. Development Workflow

### Prerequisites
- Go 1.23+
- Python 3.10+ (for model validation scripts)
- Linux Kernel 5.8+ (required only for eBPF node agent, not needed for simulator development)
- Git

### Building & Testing
```bash
# Verify formatting
go fmt ./...

# Run unit tests with race detection
go test -v -race ./...

# Run the deterministic simulator CLI
go run ./cmd/simulator --seed 42 --nodes 6 --workloads 40

# Run cross-platform Python mathematical model verification
python scripts/verify_model.py
```

---

## 3. Pull Request Guidelines

1. **Title:** Use Conventional Commits format:
   - `feat(scheduler): add PSI memory pressure scoring penalty`
   - `fix(interference): prevent zero-division on empty node active set`
   - `docs(rfc): add RFC-0002 interference model specification`
2. **Testing:** All PRs must include unit tests. PRs introducing concurrency must pass `go test -race`.
3. **Documentation:** Update relevant RFCs, docstrings, and README if your change modifies behavior.

