# Project Governance

## 1. Overview

ASTRA is an open-source distributed systems project dedicated to advancing closed-loop, kernel-informed workload scheduling. This document outlines project governance, decision-making, and maintainer responsibilities.

---

## 2. Roles & Responsibilities

- **Maintainers:** Responsible for technical direction, reviewing RFCs, merging pull requests, and cutting releases. Maintainers must uphold the zero-assumption quality rule.
- **Reviewers:** Experienced contributors who review code, test reproducibility, and audit architectural consistency.
- **Contributors:** Community members submitting bug fixes, documentation improvements, RFCs, and test suites.

---

## 3. Decision-Making & RFC Process

All major architectural proposals (new probe types, scoring algorithms, Kubernetes API modifications) require a formal RFC submitted to the `rfcs/` directory.

- RFCs require consensus among at least two maintainers before implementation begins.
- Breaking changes must conform to [RFC-0008: API Versioning Policy](rfcs/RFC-0008-api-versioning.md).
