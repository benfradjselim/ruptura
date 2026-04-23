# OHE v5.1.0 — Comprehensive Production-Readiness Review

## 1. Executive Summary
This document provides a deep-dive analysis of the MLOps Crew Automation (OHE) codebase. While the platform architecture is robust (B+ grade), significant technical debt exists. This document acts as the **System Prompt** for all future AI agents and contributors to transform OHE into a production-ready system.

---

## 2. Deep Dive: Technical Debt & Identified Issues

### A. Security Hardening (Highest Priority)
*   **JWT Secret Management:** The current implementation allows an empty JWT secret. 
    *   *Refactor Required:* Implement `crypto/rand` to generate a 32-byte secret if `OHE_JWT_SECRET` is missing in production.
*   **API Input Validation:** API handlers lack schema validation. 
    *   *Refactor Required:* Implement a centralized validation middleware for all request payloads (dashboard names, numeric bounds, user credentials).
*   **Key Injection (SQLi-like risk):** 
    *   *Issue:* Sanitization is inconsistent. `host` or `metric` names could contain ':' or '/' allowing key space injection.
    *   *Refactor Required:* Strict enforcement of `sanitizeKeySegment()` on *all* user-supplied key segments.
*   **SSRF Protection:** 
    *   *Issue:* The proxy handler (`handlers_proxy.go`) lacks validated filtering of `OHE_TRUSTED_DATASOURCE_HOSTS`.
    *   *Refactor Required:* Validate all outgoing proxy requests against the allowlist before execution.

### B. Testing Infrastructure (The Critical Gap)
*   **Coverage Crisis:** Most core internal packages (`/internal/analyzer`, `/internal/api`, `/internal/collector`) lack `*_test.go` files.
*   **Regression Risk:** Without unit tests, refactoring core storage logic (BadgerDB keys) is highly dangerous.
*   **Requirement:** 
    *   Implement unit tests for all logic. 
    *   Target: **>70%** coverage for core components.
    *   Must add: `collector_test.go` (mocking `/proc`), `storage_test.go` (CRUD/TTL/Range queries), `api_handlers_test.go` (Happy path + error paths).

### C. Code Quality & Performance Improvements
*   **Error Handling:** Current code exhibits "silent failures" (e.g., `collector/system.go` ignores errors on `readCPUStat`).
    *   *Refactor:* All errors must be logged, or returned and handled.
*   **Algorithmic Performance:** 
    *   *Issue:* `storage.go` implements an $O(n^2)$ Bubble Sort for tracing headers.
    *   *Refactor:* Replace with `sort.Slice` ($O(n \log n)$).
*   **Magic Numbers:** Scattered literals (e.g., `storage.go:86`, `100` for prefetch) need to be moved to documented constants.
*   **Dependency Management:** `go.mod` specifies Go 1.18, but Dockerfile uses 1.21. Standardize on the latest stable Go version.

### D. Operational & Deployment Debt
*   **K8s Strategy:** Deployment uses `Recreate` strategy (no zero-downtime).
    *   *Refactor:* Document limitations or implement a reader/writer splitting approach for HA.
*   **Hardcoded Configuration:**
    *   *Issue:* Hardcoded image `ghcr.io/benfradjselim/ohe:4.0.0` in `operator/controller.go`.
    *   *Fix:* Inject via CRD/env.
*   **Self-Monitoring:**
    *   *Issue:* OHE does not expose Prometheus metrics for its own performance (Goroutines, memory, query latency).
    *   *Fix:* Implement `/metrics` endpoint for OHE internals.

---

## 3. System Instructions for AI Agents
You are a **Staff Observability Engineer**. When working on OHE, you must:

1.  **Read `docs/DESIGN_SYSTEM.md` & `docs/REVIEW.md`** before starting *any* task.
2.  **No "Silent Failures":** Never use `_ =` to ignore an error without explicit, logged justification.
3.  **Validation First:** If editing an API handler, implement validation logic *before* implementation logic.
4.  **Test Requirement:** If you modify a package without tests, you **must** create or improve its `*_test.go` file.
5.  **Documentation:** Keep this roadmap updated. If you complete a task, cross it off.

---

## 4. Prioritized Roadmap

| Task | Priority | Category |
| :--- | :--- | :--- |
| **Unit Tests (Baseline)** | Critical | Testing |
| **Input Validation Middleware** | Critical | Security |
| **JWT Secret Hardening** | Critical | Security |
| **SSRF Proxy Protection** | Critical | Security |
| **Fix Bubble Sort ($O(n^2)$)** | High | Performance |
| **Implement Structured Logging (`slog`)**| High | Operations |
| **Fill remaining `docs/rnd/` files** | High | Docs |
| **Prometheus Self-Metrics** | Medium | Operations |

---

*This document is the system prompt for OHE development.*
