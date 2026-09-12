# AvtoFast Backend Core

Production-grade, high-performance Go backend service for the **AvtoFast** driving theory learning platform, strictly adhering to the [`BACKEND_API_RELEASE.md`](./docs/BACKEND_API_RELEASE.md) contract.

Built with **Go**, **Fiber v2**, **PostgreSQL** (`pgx/v5`), and **Redis** (`go-redis/v9`). Designed to handle >50,000 requests per second on an 8-core CPU / 8GB RAM node.

---

## 1. Architectural Highlights

- **Clean Architecture**: Decoupled layers separating domain entities, use cases, persistence interfaces, and HTTP transport.
- **BIGSERIAL Primary Keys**: Integer `BIGSERIAL PRIMARY KEY` internal IDs for fast 8-byte joins, compact B-Trees, and zero UUID index fragmentation, paired with opaque public string identifiers (`usr_...`, `ps_...`, `exam_...`, `evt_...`) for the external API client.
- **Strict Answer Protection**: Correct answers are omitted from ordinary question listing endpoints. Practice sessions evaluate answers on the server, while timed mock exams persist answers and grade submissions strictly against stored server questions.
- **Ed25519 Signed Content Manifests**: Offline question packs are digitally signed with Ed25519 and verified with SHA-256 checksums before delivery.
- **Idempotency**: 24-hour response caching for mutating calls with `Idempotency-Key: <uuid>`.
- **Sliding-Window Rate Limiting**: Redis-backed token bucket returning `429` with `Retry-After`.
- **Zero-Allocation JSON**: Fiber configured with `github.com/goccy/go-json` for low latency and high concurrency.

---

## 2. Directory Structure

```
.
├── .agents/                          # AI custom rules and skills
│   ├── rules/                        # Performance, Architecture, and Testing guidelines
│   └── skills/                       # Workflows for performance, migrations, content authoring
├── api/
│   └── openapi.yaml                  # Complete OpenAPI 3.1 contract specification
├── bin/                              # Compiled production binaries
├── cmd/
│   ├── api/main.go                   # Main HTTP API server entrypoint
│   ├── migrate/main.go               # Database migration runner CLI
│   └── seed/main.go                  # Question pack and translations seeder
├── config/
│   └── config.yaml                   # Application configuration template
├── internal/
│   ├── config/config.go              # Environment-aware config loader
│   ├── delivery/http/
│   │   ├── handler/                  # HTTP handlers for P0, P1/P2, and Admin
│   │   ├── middleware/               # Auth, Tracing, Idempotency, RateLimit, Headers
│   │   └── router/router.go          # Route definitions & mounting under /v1
│   ├── domain/                       # Pure domain models and repository interfaces
│   ├── repository/
│   │   ├── memory/                   # Thread-safe concurrent in-memory store for instant tests
│   │   ├── postgres/                 # High-performance pgxpool PostgreSQL repository
│   │   └── redis/                    # Redis caching, rate limiting, and idempotency store
│   └── usecase/                      # Core business logic implementations
├── migrations/
│   ├── 000001_init_schema.up.sql     # Database schema with BIGSERIAL primary keys
│   └── 000001_init_schema.down.sql   # Rollback schema
├── pkg/
│   ├── crypto/                       # Ed25519 signing and SHA256 checksums
│   ├── id/                           # Fast opaque prefixed ULID generator
│   ├── jwt/                          # Provider-agnostic JWT verifier & test token generator
│   ├── logger/                       # Structured JSON logger without sensitive leaks
│   └── response/                     # RFC-compliant error envelopes
└── test/
    ├── benchmark/                    # Serialization and HTTP throughput benchmarks
    └── e2e/                          # End-to-end integration tests for all P0 flows
```

---

## 3. Quick Start

### Build Binaries
```bash
make build
```

### Run Tests
```bash
# Run all unit and E2E tests
make test

# Run with race detector
make test-race
```

### Run Benchmarks
```bash
make benchmark
```

### Start Server
```bash
make run
```

---

## 4. Endpoint Summary

### P0 Endpoints (`/v1`)
| Method | Path | Auth | Purpose |
| --- | --- | --- | --- |
| GET | `/v1/bootstrap` | optional | App configuration, active pack versions, flags, pass threshold |
| GET | `/v1/me` | user | Profile, settings, onboarding state, entitlement summary |
| PATCH | `/v1/me` | user | Avatar/profile metadata and privacy settings |
| PUT | `/v1/me/onboarding` | user | Save source, knowledge level, language, target date, daily goal |
| PATCH | `/v1/me/preferences` | user | Language, theme, profile & leaderboard visibility |
| PATCH | `/v1/me/notification-preferences` | user | Reminder, streak, weekly-summary, result settings |
| GET | `/v1/content/packs` | user | Pack catalogue and version availability |
| GET | `/v1/content/packs/:packId/manifest` | user | Signed metadata and checksum |
| GET | `/v1/content/packs/:packId/questions` | user | Online questions (correctChoiceId omitted) |
| POST | `/v1/content/packs/:packId/offline-download` | user | Short-lived signed URL for offline pack |
| POST | `/v1/practice-sessions` | user | Create adaptive/category/mistake practice session |
| POST | `/v1/practice-sessions/:sessionId/answers` | user | Submit and evaluate one practice answer |
| POST | `/v1/practice-sessions/:sessionId/complete` | user | Finalize practice and award progress |
| GET | `/v1/review/mistakes` | user | Scheduled incorrect questions for repeat study |
| POST | `/v1/mock-exams` | user | Create official-format timed mock exam |
| PUT | `/v1/mock-exams/:examId/answers/:questionId` | user | Persist an in-progress answer (no feedback) |
| POST | `/v1/mock-exams/:examId/submit` | user | Server-grade score, pass/fail, and detailed review |
| GET | `/v1/mock-exams` | user | Paginated attempt history |
| GET | `/v1/dashboard` | user | Streak, XP, readiness, goals, category performance |
| GET | `/v1/analytics/progress` | user | Time series and category accuracy history |
| POST | `/v1/sync/events` | user | Idempotent offline progress upload |
| GET | `/v1/sync/changes` | user | Cursor-based changes to merge on client |
| GET | `/v1/entitlements` | user | Premium entitlement status |
| POST | `/v1/billing/verify` | user | Server-side Apple/Google purchase verification |
| POST | `/v1/billing/restore` | user | Reconcile purchases after restore action |
| PUT | `/v1/devices/:deviceId/push-token` | user | Register/revoke FCM/APNs push token |

### Admin Endpoints (`/v1/admin`)
| Method | Path | Role | Purpose |
| --- | --- | --- | --- |
| POST | `/v1/admin/content/packs` | content_admin | Create a draft pack/version |
| POST | `/v1/admin/content/packs/:packId/questions` | content_admin | Add draft question, translations, image and source |
| PATCH | `/v1/admin/questions/:questionId` | content_admin | Edit draft content |
| POST | `/v1/admin/questions/:questionId/validate` | content_admin | Validate all 4 locales, choices, source, image |
| POST | `/v1/admin/content/packs/:packId/publish` | content_publisher | Publish immutable version, sign manifest, invalidate cache |
| POST | `/v1/admin/content/packs/:packId/rollback` | content_publisher | Rollback to prior version |
| GET | `/v1/admin/audit-log` | content_publisher | Read content/publish change audit entries |
