# Clean Architecture & Engineering Standards

This document establishes the architecture rules and boundaries for the AvtoFast codebase.

---

## 1. Clean Architecture Layers

Dependencies must point inwards only:
`Delivery (HTTP)` ➔ `Usecase` ➔ `Domain (Core)` 
`Repository (Postgres/Redis)` ➔ `Domain (Core)`

1. **`internal/domain` (Core Layer)**:
   - Contains pure entity structs, domain error constants, and repository/store interfaces.
   - **Zero external dependencies**: Must not import Fiber, PGX, Redis, or any third-party frameworks.
2. **`internal/usecase` (Business Logic Layer)**:
   - Implements application workflows: practice sessions, mock exams, dashboard statistics, spaced repetition, offline sync, and billing verification.
   - Depends only on `domain` repository interfaces.
3. **`internal/repository` (Data Layer)**:
   - `postgres/`: Implements domain repositories using `pgxpool` with parameterized SQL.
   - `redis/`: Implements caching, 24-hour idempotency, and sliding-window rate limiting.
   - `memory/`: Complete thread-safe in-memory store for unit and E2E contract testing without external daemons.
4. **`internal/delivery/http` (Transport Layer)**:
   - Handlers, DTOs, request binders, and middleware (Auth, Request Tracing, Idempotency, Rate Limiting, Header Validation).
   - Maps domain errors to standard RFC-compliant JSON error envelopes.

---

## 2. Database Schema Conventions

- **Primary Keys**: Every table MUST use `id BIGSERIAL PRIMARY KEY`.
- **Foreign Keys**: Must reference the integer `BIGINT` primary key on target tables for maximum join performance.
- **External Public IDs**: Store prefixed opaque ULIDs (`usr_`, `ps_`, `exam_`, `evt_`, `req_`) in `public_id VARCHAR(64) UNIQUE NOT NULL`.
- **Audit Columns**: Include `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` and `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.

---

## 3. Answer Privacy & Exam Integrity

- **Never reveal `correctChoiceId`** in question catalogue endpoints (`GET /v1/content/packs/:packId/questions`).
- Answer evaluation MUST happen on the server:
  - Online Practice: Evaluate and return immediate explanation, XP, and mistake update.
  - Timed Mock Exams: Record answers in progress; only grade and evaluate once submitted (`POST /v1/mock-exams/:examId/submit`).
  - Reject exam answers and submissions with `409 EXAM_EXPIRED` if the server deadline has passed.

---

## 4. Error Envelope Standard

All non-2xx responses must use this exact structure:
```json
{
  "error": {
    "code": "ERROR_CODE_SNAKE_OR_SCREAMING",
    "message": "Human-readable explanation.",
    "requestId": "req_01J...",
    "details": { ... }
  }
}
```
HTTP status codes:
- `400`: Validation error (`VALIDATION_ERROR`, `INVALID_BODY`, `INVALID_DATE`)
- `401`: Missing or invalid Bearer token (`UNAUTHORIZED`)
- `403`: Role or feature flag denial (`FORBIDDEN`, `FEATURE_DISABLED`)
- `404`: Resource not found (`NOT_FOUND`)
- `409`: Conflict, duplicate, or expired state (`EXAM_EXPIRED`, `CONFLICT`)
- `422`: Unprocessable entity
- `429`: Rate limit exceeded (`RATE_LIMIT_EXCEEDED` with `Retry-After` header)
- `500`: Internal server error
