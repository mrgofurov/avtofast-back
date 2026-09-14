# AGENTS.md — AvtoFast Backend AI Collaboration & System Guide

This document defines the architecture, conventions, technology stack, and engineering rules for AI agents and human engineers working on the **AvtoFast** backend.

---

## 1. Project Overview & Business Domain

**AvtoFast** is a high-performance driving theory learning platform tailored for Uzbekistan driver examination preparation.
- **Production API Domain**: `https://api.avtofast.uz/v1`
- **Supported Languages**: Uzbek Latin (`uz-Latn-UZ`), Uzbek Cyrillic (`uz-Cyrl-UZ`), Russian (`ru`), English (`en`).
- **Target Performance**: **>50,000 requests per second (RPC)** on an 8-core CPU / 8GB RAM / 100GB SSD production node.

---

## 2. Technology Stack

| Layer | Technology | Rationale |
| --- | --- | --- |
| **Language** | Go 1.26+ | Fast compiled binary, low latency, strong concurrency primitives. |
| **HTTP Framework** | Fiber v2 (`valyala/fasthttp`) | Zero memory allocation routing and connection management. |
| **JSON Serializer** | `goccy/go-json` | Ultra fast zero/low alloc JSON marshaling/unmarshaling in Fiber. |
| **Relational Database** | PostgreSQL 16+ | ACID relational store with binary protocol driver. |
| **PostgreSQL Driver** | `jackc/pgx/v5` (`pgxpool`) | Connection pooling, prepared statement caching, binary encoding. |
| **Cache & In-Memory Store** | Redis 7+ (`go-redis/v9`) | Hot endpoint caching, 24h idempotency response cache, sliding-window rate limiting. |
| **Auth / Cryptography** | `golang-jwt/jwt/v5`, `crypto/ed25519` | Provider-agnostic JWT verification and offline content pack digital signing. |
| **ID Generation** | `oklog/ulid/v2` | Fast, monotonic, opaque prefixed ULIDs (`usr_`, `ps_`, `exam_`, `evt_`, `req_`). |

---

## 3. Clean Code Architecture

The project follows Clean / Hexagonal Architecture:

```
├── cmd/
│   ├── api/main.go            # Entry point, dependency wiring, graceful shutdown
│   ├── migrate/main.go        # Database migrations runner CLI
│   └── seed/main.go           # Initial question pack & 4-locale translations seeder
├── config/                    # Default YAML config templates
├── internal/
│   ├── config/                # Environment-aware config loader (.env via godotenv)
│   ├── domain/                # Pure domain entities, repository & store interfaces
│   ├── usecase/               # Pure business logic (Auth, Practice, Exam, Dashboard, Sync)
│   ├── repository/
│   │   ├── memory/            # Thread-safe in-memory store for instant zero-flake tests
│   │   ├── postgres/          # pgxpool-backed PostgreSQL persistence with BIGSERIAL
│   │   └── redis/             # Redis caching, idempotency, and token-bucket rate limiter
│   └── delivery/http/
│       ├── handler/           # Fiber request handlers
│       ├── middleware/        # Auth, Request Tracing, Idempotency, Rate Limiting, Headers
│       └── router/            # Route mounting under /v1
├── migrations/                # Versioned SQL migration files (.up.sql, .down.sql)
├── pkg/                       # Reusable utility packages (crypto, id, jwt, logger, response)
└── test/                      # E2E test suite and high-throughput benchmark tests
```

---

## 4. Fundamental Rules & Conventions

### 4.1. Database Schema Standards
- **Primary Keys**: Always use `id BIGSERIAL PRIMARY KEY` (or `BIGINT GENERATED ALWAYS AS IDENTITY`).
  - Integer joins are 8 bytes, keeping indexes compact and preventing UUID B-Tree fragmentation.
  - Foreign keys between tables MUST use `BIGINT REFERENCES ... (id)`.
- **Public Identifiers**: Store opaque prefixed string IDs in `public_id VARCHAR(64) UNIQUE NOT NULL`.
  - Expose `public_id` as `"id"` in JSON API responses (`"id": "usr_01J..."`).
  - Never expose internal database sequence integers to API clients.

### 4.2. Answer Privacy & Security
- **Never expose `correctChoiceId`** in ordinary question-list or question-bank responses (`GET /v1/content/packs/:packId/questions`).
- **Server-Side Evaluation**:
  - In practice sessions (`POST /v1/practice-sessions/:sessionId/answers`), evaluate choice correctness server-side and record mistakes.
  - In timed mock exams, store selected choices without immediate feedback. Grade scores strictly from server-held questions on submission (`POST /v1/mock-exams/:examId/submit`).
  - Reject exam submissions after the deadline with `409 EXAM_EXPIRED`.

### 4.3. High-Performance Design (Target: 50k RPC)
- Use `goccy/go-json` for all JSON encoding/decoding.
- Avoid reflection and heavy memory allocations inside hot HTTP request paths.
- Ensure composite indexes exist for user-scoped time-series queries (e.g. `(user_id, created_at DESC)`).
- Use `pgxpool` with prepared statements enabled (`StatementCacheCapacity: 512`).
- Store idempotency keys in Redis for mutating endpoints (`POST`, `PUT`, `PATCH`) with a 24-hour TTL.

### 4.4. Offline Content Verification
- Question packs for offline use must have an Ed25519 digital signature of the manifest SHA-256 checksum.
- Mobile clients verify the signature before activating local question packs.

---

## 5. Development & Testing Commands

```bash
# Build production binaries to bin/
make build

# Run all unit and integration tests
make test

# Run tests under Go's race detector
make test-race

# Run performance and memory allocation benchmarks
make benchmark

# Run database migrations
make migrate-up
make migrate-down

# Seed initial question pack & translations
make seed

# Run server locally
make run
```
