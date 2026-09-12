# Testing & Quality Assurance Standards

This document specifies the testing rules and validation requirements for the AvtoFast backend.

---

## 1. Test Categories & Expectations

1. **Unit Tests (`internal/usecase/*_test.go`, `pkg/*/*_test.go`)**:
   - Must execute in < 1 second.
   - Must cover boundary cases: expired tokens, invalid dates, past target dates, missing translations, and spaced repetition intervals.
2. **Race Detector Mandatory**:
   - All tests must pass with `go test -v -race ./...`.
   - Any data race is considered a blocking release failure.
3. **End-to-End (E2E) Contract Tests (`test/e2e/e2e_test.go`)**:
   - Uses Fiber's `app.Test(req, timeout)` to exercise the entire HTTP middleware and router pipeline.
   - Tests authentication, header validation (`X-Platform`, `Accept-Language`), idempotency 24h caching, server-graded mock exams, and admin role enforcement.
   - Must be fully self-contained and run without external daemon dependencies.

---

## 2. Command Reference

```bash
# Run all tests
make test

# Run all tests under race detector
make test-race

# Run performance benchmarks with memory allocations
make benchmark
```
