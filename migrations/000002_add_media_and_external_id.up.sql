-- 000002_add_media_and_external_id.up.sql
-- Add video_url, audio_url, and external_id to questions table for multimedia explanations

ALTER TABLE questions ADD COLUMN IF NOT EXISTS video_url TEXT;
ALTER TABLE questions ADD COLUMN IF NOT EXISTS audio_url TEXT;
ALTER TABLE questions ADD COLUMN IF NOT EXISTS external_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_questions_external_id ON questions(external_id);
