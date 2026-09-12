---
name: performance-optimization
description: Workflows for profiling, benchmarking, and optimizing high-throughput Fiber and Postgres operations for 50k RPC.
---

# Performance Optimization Skill

This guide outlines actionable workflows to maintain and improve backend throughput up to 50k+ RPC on an 8-core CPU / 8GB RAM machine.

---

## 1. Running Benchmarks

Run benchmarks reporting memory allocation count and bytes:
```bash
go test -bench=. -benchmem ./test/benchmark/...
```

### Interpretation Targets:
- **Handler serialization**: < 500 ns/op, < 5 allocs/op.
- **HTTP endpoint throughput**: > 50,000 requests/second under parallel execution.

---

## 2. Profiling CPU and Memory with pprof

Generate CPU profile:
```bash
go test -bench=BenchmarkBootstrapEndpoint -cpuprofile=cpu.pprof ./test/benchmark/...
go tool pprof -http=:8081 cpu.pprof
```

Generate Memory allocation profile:
```bash
go test -bench=BenchmarkBootstrapEndpoint -memprofile=mem.pprof ./test/benchmark/...
go tool pprof -http=:8082 mem.pprof
```

---

## 3. Optimization Checklist for New Endpoints

- [ ] Does the endpoint parse JSON using `goccy/go-json`?
- [ ] Are SQL queries utilizing prepared statement cache?
- [ ] Are foreign key joins using 8-byte `BIGINT` integer columns rather than string UUIDs?
- [ ] Are all batch lookups using `ANY($1)` instead of iterative queries inside loops?
- [ ] Is mutating state protected with Redis idempotency caching (`24h TTL`)?
- [ ] Is the endpoint covered by a benchmark in `test/benchmark/`?
