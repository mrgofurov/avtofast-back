-- 000005_add_tickets.up.sql
-- Tickets ("biletlar"): the numbered 20-question sets learners know from the
-- exam centres and from Prepdrive's templates. Unlike topic tests, which are
-- slices of one category, a ticket mixes topics in a fixed order, and the
-- same question may sit in more than one ticket — so a ticket is a list of
-- question references, not a range.

CREATE TABLE IF NOT EXISTS tickets (
    id          BIGSERIAL PRIMARY KEY,
    pack_id     VARCHAR(64) NOT NULL,
    number      INT NOT NULL,
    -- Where the ticket came from, e.g. "prepdrive:12", for re-imports.
    source      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (pack_id, number)
);

CREATE TABLE IF NOT EXISTS ticket_questions (
    ticket_id   BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    position    INT NOT NULL,
    question_id BIGINT NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    PRIMARY KEY (ticket_id, position)
);

-- Which ticket a practice session was; NULL for every other kind.
ALTER TABLE practice_sessions ADD COLUMN IF NOT EXISTS ticket_number INT;

CREATE INDEX IF NOT EXISTS idx_practice_tickets
    ON practice_sessions(user_id, pack_id, ticket_number)
    WHERE ticket_number IS NOT NULL;
