# Performance & High-Throughput Engineering Rules (>50,000 RPC)

To guarantee that the backend comfortably sustains 50k+ RPC on an 8-core CPU / 8GB RAM node, all code must adhere to the following performance directives.

---

## 1. Zero/Low Allocation in Hot Paths

1. **JSON Handling**:
   - Always use `github.com/goccy/go-json` configured in Fiber (`fiber.Config{ JSONEncoder: json.Marshal, JSONDecoder: json.Unmarshal }`).
   - Avoid parsing into generic `map[string]any` when structured DTOs can be used.
2. **String and Byte Operations**:
   - Avoid unnecessary allocations between `[]byte` and `string`. Use `c.Request().Body()` and Fasthttp zero-copy utilities where safe.
   - For string formatting in high-frequency paths, prefer `strconv.Append...` or pre-allocated byte buffers over excessive `fmt.Sprintf` calls.
3. **Pool Recycling**:
   - Use `sync.Pool` for heavy reusable objects like random number readers, buffer writers, and hasher states (see `pkg/id/id.go`).

---

## 2. PostgreSQL Query Optimization (`pgxpool`)

1. **Connection Pooling**:
   - Keep maximum connections bounded to hardware capacity: `MaxConns: 80`, `MinConns: 20` on 8-core hardware to prevent thread starvation and lock thrashing.
   - Enable prepared statement caching (`StatementCacheCapacity: 512`) to eliminate query parsing overhead on repeated executions.
2. **Prevent N+1 Queries**:
   - Never issue SQL queries inside a loop.
   - Batch load related records (e.g. question translations or practice questions) using PostgreSQL array matching: `WHERE question_id = ANY($1)`.
3. **BIGSERIAL Index Compactness**:
   - Internal table joins and foreign keys MUST use 8-byte `BIGINT` references.
   - Create composite B-Tree indexes for user-scoped time-series queries:
     - `CREATE INDEX idx_user_exams ON mock_exams(user_id, started_at DESC);`
     - `CREATE INDEX idx_questions_pack_cat ON questions(pack_id, category);`
     - `CREATE INDEX idx_user_mistakes_review ON user_mistakes(user_id, next_review_at);`

---

## 3. Multi-Tier Caching & Redis Optimization

1. **Hot Content Caching**:
   - Immutable published content (such as active question pack manifests and `/bootstrap`) should be cached with client cache headers (`Cache-Control: public, max-age=900`) and cached in memory.
2. **Redis Pipelining**:
   - In rate limiting and multi-key session updates, use Redis `TxPipeline()` to execute commands in a single network round-trip.
3. **Idempotency Key TTL**:
   - All mutating API calls accept `Idempotency-Key: <uuid>`.
   - Store cached responses in Redis under `idemp:<user_id>:<key>` with a 24-hour expiration.

---

## 4. Benchmark Enforcement

Every performance-critical path must have a corresponding benchmark test in `test/benchmark/`:
```bash
go test -bench=. -benchmem ./test/benchmark/...
```
Allocations per operation in handlers must be minimized (< 100 allocs/op for full HTTP roundtrips).
