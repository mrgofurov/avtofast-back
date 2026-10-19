-- 000005_add_tickets.down.sql
DROP INDEX IF EXISTS idx_practice_tickets;
ALTER TABLE practice_sessions DROP COLUMN IF EXISTS ticket_number;
DROP TABLE IF EXISTS ticket_questions;
DROP TABLE IF EXISTS tickets;
