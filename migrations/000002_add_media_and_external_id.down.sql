-- 000002_add_media_and_external_id.down.sql
DROP INDEX IF EXISTS idx_questions_external_id;
ALTER TABLE questions DROP COLUMN IF EXISTS external_id;
ALTER TABLE questions DROP COLUMN IF EXISTS audio_url;
ALTER TABLE questions DROP COLUMN IF EXISTS video_url;
