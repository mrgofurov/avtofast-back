-- 000003_add_topic_tests.down.sql

DROP INDEX IF EXISTS idx_questions_pack_cat_seq;
DROP INDEX IF EXISTS idx_practice_topic_tests;
ALTER TABLE practice_sessions DROP COLUMN IF EXISTS test_index;
