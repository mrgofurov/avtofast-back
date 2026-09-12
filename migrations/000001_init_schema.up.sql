-- 000001_init_schema.up.sql
-- High-performance schema for AvtoFast with BIGSERIAL primary keys

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL,
    provider_id VARCHAR(128) NOT NULL,
    provider VARCHAR(32) NOT NULL DEFAULT 'firebase',
    email VARCHAR(255),
    phone VARCHAR(32),
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url TEXT,
    role VARCHAR(32) NOT NULL DEFAULT 'user', -- user, content_admin, content_publisher
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id);
CREATE INDEX IF NOT EXISTS idx_users_public_id ON users(public_id);

-- User onboarding table
CREATE TABLE IF NOT EXISTS user_onboardings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    acquisition_source VARCHAR(64) NOT NULL DEFAULT 'other',
    knowledge_level VARCHAR(32) NOT NULL DEFAULT 'beginner',
    locale VARCHAR(32) NOT NULL DEFAULT 'uz-Latn-UZ',
    target_exam_date DATE NOT NULL,
    daily_question_goal INT NOT NULL DEFAULT 10,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- User preferences table
CREATE TABLE IF NOT EXISTS user_preferences (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    locale VARCHAR(32) NOT NULL DEFAULT 'uz-Latn-UZ',
    theme VARCHAR(16) NOT NULL DEFAULT 'system',
    profile_visibility VARCHAR(16) NOT NULL DEFAULT 'friends',
    leaderboard_visibility VARCHAR(16) NOT NULL DEFAULT 'friends',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Notification preferences table
CREATE TABLE IF NOT EXISTS notification_preferences (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    study_reminder BOOLEAN NOT NULL DEFAULT TRUE,
    streak_protection BOOLEAN NOT NULL DEFAULT TRUE,
    mistake_review BOOLEAN NOT NULL DEFAULT TRUE,
    weekly_summary BOOLEAN NOT NULL DEFAULT TRUE,
    score_improvement BOOLEAN NOT NULL DEFAULT TRUE,
    reminder_time VARCHAR(8) NOT NULL DEFAULT '19:00',
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Tashkent',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- User devices table
CREATE TABLE IF NOT EXISTS devices (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL, -- installation UUID
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(16) NOT NULL, -- ios | android
    push_token TEXT,
    app_version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    locale VARCHAR(32) NOT NULL DEFAULT 'uz-Latn-UZ',
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Tashkent',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_devices_user_id ON devices(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_public_id ON devices(public_id);

-- Entitlements table
CREATE TABLE IF NOT EXISTS entitlements (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tier VARCHAR(32) NOT NULL DEFAULT 'free', -- free | premium
    status VARCHAR(32) NOT NULL DEFAULT 'active', -- active | expired | canceled
    product_id VARCHAR(128),
    expires_at TIMESTAMPTZ,
    features JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Purchases / Receipts table
CREATE TABLE IF NOT EXISTS purchases (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(16) NOT NULL, -- ios | android
    transaction_id VARCHAR(255) UNIQUE NOT NULL,
    product_id VARCHAR(128) NOT NULL,
    raw_payload TEXT NOT NULL DEFAULT '',
    verified_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_purchases_user ON purchases(user_id);

-- Question packs table
CREATE TABLE IF NOT EXISTS question_packs (
    id BIGSERIAL PRIMARY KEY,
    pack_id VARCHAR(64) NOT NULL, -- e.g. 'uz-theory-2026-09'
    version VARCHAR(32) NOT NULL, -- e.g. '2026.09.1'
    title VARCHAR(255) NOT NULL,
    locales TEXT[] NOT NULL DEFAULT ARRAY['uz-Latn-UZ','uz-Cyrl-UZ','ru','en'],
    question_count INT NOT NULL DEFAULT 0,
    download_bytes BIGINT NOT NULL DEFAULT 0,
    mandatory_update BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(32) NOT NULL DEFAULT 'draft', -- draft, published, deprecated
    manifest_sha256 VARCHAR(128) NOT NULL DEFAULT '',
    manifest_signature TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_packs_pack_id_version ON question_packs(pack_id, version);
CREATE INDEX IF NOT EXISTS idx_packs_status ON question_packs(status);

-- Questions table
CREATE TABLE IF NOT EXISTS questions (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL, -- e.g. 'sign-014' or 'cross-027'
    pack_table_id BIGINT NOT NULL REFERENCES question_packs(id) ON DELETE CASCADE,
    pack_id VARCHAR(64) NOT NULL,
    content_version VARCHAR(32) NOT NULL,
    category VARCHAR(64) NOT NULL, -- road_signs, traffic_rules, intersections, first_aid, penalties, vehicle_safety, situations
    difficulty VARCHAR(16) NOT NULL DEFAULT 'easy', -- easy, medium, hard
    image_url TEXT,
    image_sha256 VARCHAR(128),
    image_alt JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_reference TEXT NOT NULL DEFAULT '',
    source_effective_from DATE,
    source_official_url TEXT,
    correct_choice_id VARCHAR(16) NOT NULL, -- server-side only
    status VARCHAR(32) NOT NULL DEFAULT 'published', -- published, deprecated, draft
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_questions_pack_cat ON questions(pack_id, category);
CREATE INDEX IF NOT EXISTS idx_questions_pack_id ON questions(pack_id);
CREATE INDEX IF NOT EXISTS idx_questions_category ON questions(category);

-- Question translations table
CREATE TABLE IF NOT EXISTS question_translations (
    id BIGSERIAL PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    locale VARCHAR(32) NOT NULL,
    prompt TEXT NOT NULL,
    choices JSONB NOT NULL, -- [{"id":"a","text":"...","position":1},...]
    explanation TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_translations_q_locale ON question_translations(question_id, locale);

-- Practice sessions table
CREATE TABLE IF NOT EXISTS practice_sessions (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL, -- 'ps_...'
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mode VARCHAR(32) NOT NULL, -- adaptive, category, mistake_review, daily_goal
    pack_id VARCHAR(64) NOT NULL,
    pack_version VARCHAR(32) NOT NULL,
    locale VARCHAR(32) NOT NULL,
    category VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'in_progress', -- in_progress, completed
    total_questions INT NOT NULL DEFAULT 0,
    answered_count INT NOT NULL DEFAULT 0,
    correct_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_practice_user ON practice_sessions(user_id, created_at DESC);

-- Practice session question assignments
CREATE TABLE IF NOT EXISTS practice_session_questions (
    id BIGSERIAL PRIMARY KEY,
    session_id BIGINT NOT NULL REFERENCES practice_sessions(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    order_index INT NOT NULL,
    selected_choice_id VARCHAR(16),
    is_correct BOOLEAN,
    elapsed_ms INT NOT NULL DEFAULT 0,
    answered_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_psq_session ON practice_session_questions(session_id, order_index);
CREATE UNIQUE INDEX IF NOT EXISTS idx_psq_session_question ON practice_session_questions(session_id, question_id);

-- Mock exams table
CREATE TABLE IF NOT EXISTS mock_exams (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL, -- 'exam_...'
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pack_id VARCHAR(64) NOT NULL,
    pack_version VARCHAR(32) NOT NULL,
    locale VARCHAR(32) NOT NULL,
    format VARCHAR(32) NOT NULL DEFAULT 'official_20',
    status VARCHAR(32) NOT NULL DEFAULT 'in_progress', -- in_progress, completed, expired
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline_at TIMESTAMPTZ NOT NULL,
    question_count INT NOT NULL DEFAULT 20,
    pass_correct_count INT NOT NULL DEFAULT 16,
    correct_count INT NOT NULL DEFAULT 0,
    score_percent INT NOT NULL DEFAULT 0,
    passed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_mock_exams_user ON mock_exams(user_id, started_at DESC);

-- Mock exam questions table
CREATE TABLE IF NOT EXISTS mock_exam_questions (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT NOT NULL REFERENCES mock_exams(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    order_index INT NOT NULL,
    selected_choice_id VARCHAR(16),
    is_correct BOOLEAN,
    elapsed_ms INT NOT NULL DEFAULT 0,
    answered_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_meq_exam ON mock_exam_questions(exam_id, order_index);
CREATE UNIQUE INDEX IF NOT EXISTS idx_meq_exam_q ON mock_exam_questions(exam_id, question_id);

-- User mistakes / spaced-repetition review queue
CREATE TABLE IF NOT EXISTS user_mistakes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    mistake_count INT NOT NULL DEFAULT 1,
    consecutive_correct INT NOT NULL DEFAULT 0,
    last_incorrect_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    next_review_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    priority VARCHAR(16) NOT NULL DEFAULT 'high', -- high, medium, low
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_mistakes_user_q ON user_mistakes(user_id, question_id);
CREATE INDEX IF NOT EXISTS idx_user_mistakes_review ON user_mistakes(user_id, next_review_at);

-- User stats & progression (for fast 50k RPC dashboard lookups)
CREATE TABLE IF NOT EXISTS user_stats (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    xp INT NOT NULL DEFAULT 0,
    level INT NOT NULL DEFAULT 1,
    streak_days INT NOT NULL DEFAULT 0,
    last_activity_date DATE,
    total_answered INT NOT NULL DEFAULT 0,
    total_correct INT NOT NULL DEFAULT 0,
    completed_mock_exams INT NOT NULL DEFAULT 0,
    passed_mock_exams INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Offline Sync events table
CREATE TABLE IF NOT EXISTS sync_events (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL, -- 'evt_...'
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type VARCHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    pack_id VARCHAR(64),
    pack_version VARCHAR(32),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_sync_events_user ON sync_events(user_id, synced_at DESC);

-- Admin Audit logs
CREATE TABLE IF NOT EXISTS admin_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(64) UNIQUE NOT NULL,
    actor_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(64) NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id VARCHAR(64) NOT NULL,
    before_state JSONB,
    after_state JSONB,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON admin_audit_logs(actor_id, created_at DESC);
