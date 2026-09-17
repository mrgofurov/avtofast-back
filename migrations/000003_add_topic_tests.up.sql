-- 000003_add_topic_tests.up.sql
-- Numbered tests per topic: a category's published questions cut into fixed,
-- repeatable slices, so "Test 7" is the same twenty questions every time and a
-- learner can work through all of a category rather than sampling it.

-- Which slice a session was. NULL for every other mode, and for the free-form
-- category practice that predates this.
ALTER TABLE practice_sessions ADD COLUMN IF NOT EXISTS test_index INT;

-- Reading back a learner's score per test: the whole result list for one topic
-- is a single index scan.
CREATE INDEX IF NOT EXISTS idx_practice_topic_tests
    ON practice_sessions(user_id, pack_id, category, test_index)
    WHERE test_index IS NOT NULL;

-- A slice is only repeatable if the ordering behind it is. `questions.id` is
-- the insertion sequence and never changes, so it is the stable key; this index
-- lets OFFSET/LIMIT over one category walk it directly instead of sorting.
CREATE INDEX IF NOT EXISTS idx_questions_pack_cat_seq
    ON questions(pack_id, category, id)
    WHERE status = 'published';
