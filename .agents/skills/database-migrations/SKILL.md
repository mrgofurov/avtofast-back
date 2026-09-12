---
name: database-migrations
description: Workflows for schema changes, BIGSERIAL primary keys, and zero-downtime index creation in PostgreSQL.
---

# Database Migrations Skill

Guidelines and step-by-step procedures for safely evolving the AvtoFast PostgreSQL schema.

---

## 1. Migration File Structure

All migrations live under `migrations/`:
- `NNNNNN_name.up.sql`: Forward schema changes.
- `NNNNNN_name.down.sql`: Reversible rollback changes.

---

## 2. Table Creation Standard

Always follow this structure for new tables:
```sql
CREATE TABLE IF NOT EXISTS sample_table (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sample_user ON sample_table(user_id, created_at DESC);
```

### Golden Rules:
1. Internal primary keys MUST be `BIGSERIAL PRIMARY KEY`.
2. Foreign keys MUST be `BIGINT REFERENCES ... (id)`.
3. Client-facing identifiers MUST be stored in `public_id VARCHAR(64) UNIQUE NOT NULL`.

---

## 3. Applying and Rolling Back Migrations

```bash
# Apply pending up migrations
make migrate-up

# Roll back last migration
make migrate-down
```
