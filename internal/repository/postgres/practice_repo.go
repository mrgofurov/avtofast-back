package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/jackc/pgx/v5"
)

type PracticeRepository struct {
	db          *DB
	contentRepo *ContentRepository
}

func NewPracticeRepository(db *DB, contentRepo *ContentRepository) *PracticeRepository {
	return &PracticeRepository{db: db, contentRepo: contentRepo}
}

func (r *PracticeRepository) CreateSession(ctx context.Context, session *domain.PracticeSession) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	querySession := `INSERT INTO practice_sessions (public_id, user_id, mode, pack_id, pack_version, locale, category, test_index, status, total_questions, created_at)
	                 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	                 RETURNING id`
	err = tx.QueryRow(ctx, querySession,
		session.PublicID, session.UserID, session.Mode, session.PackID, session.PackVersion,
		session.Locale, session.Category, session.TestIndex, session.Status, session.TotalQuestions, session.CreatedAt,
	).Scan(&session.ID)
	if err != nil {
		return err
	}

	queryQuestion := `INSERT INTO practice_session_questions (session_id, question_id, order_index)
	                  VALUES ($1, $2, $3)`
	for i, q := range session.Questions {
		_, err := tx.Exec(ctx, queryQuestion, session.ID, q.ID, i+1)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *PracticeRepository) GetSessionByPublicID(ctx context.Context, publicID string) (*domain.PracticeSession, error) {
	query := `SELECT id, public_id, user_id, mode, pack_id, pack_version, locale, category, test_index, status,
	                 total_questions, answered_count, correct_count, created_at, completed_at
	          FROM practice_sessions WHERE public_id = $1`
	var s domain.PracticeSession
	err := r.db.Pool.QueryRow(ctx, query, publicID).Scan(
		&s.ID, &s.PublicID, &s.UserID, &s.Mode, &s.PackID, &s.PackVersion, &s.Locale,
		&s.Category, &s.TestIndex, &s.Status, &s.TotalQuestions, &s.AnsweredCount, &s.CorrectCount,
		&s.CreatedAt, &s.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Fetch ordered questions
	qQuery := `SELECT q.public_id FROM practice_session_questions psq
	           JOIN questions q ON q.id = psq.question_id
	           WHERE psq.session_id = $1 ORDER BY psq.order_index ASC`
	rows, err := r.db.Pool.Query(ctx, qQuery, s.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var qIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			return nil, err
		}
		qIDs = append(qIDs, pid)
	}

	questions, err := r.contentRepo.GetQuestionsByIDs(ctx, qIDs)
	if err != nil {
		return nil, err
	}

	// Maintain order
	qMap := make(map[string]*domain.Question)
	for _, q := range questions {
		qMap[q.PublicID] = q
	}
	for _, pid := range qIDs {
		if q, ok := qMap[pid]; ok {
			s.Questions = append(s.Questions, q)
		}
	}

	return &s, nil
}

func (r *PracticeRepository) SaveAnswer(ctx context.Context, sessionID int64, questionID int64, selectedChoiceID string, isCorrect bool, elapsedMs int, answeredAt time.Time) error {
	query := `UPDATE practice_session_questions
	          SET selected_choice_id = $1, is_correct = $2, elapsed_ms = $3, answered_at = $4
	          WHERE session_id = $5 AND question_id = $6`
	_, err := r.db.Pool.Exec(ctx, query, selectedChoiceID, isCorrect, elapsedMs, answeredAt, sessionID, questionID)
	return err
}

// GetTopicTestResults collapses every completed attempt at every numbered test
// in one category into one row per test. Aggregating in SQL rather than reading
// the sessions back keeps the topic screen a single round trip however many
// times the learner has retried.
func (r *PracticeRepository) GetTopicTestResults(ctx context.Context, userID int64, packID, category string) (map[int]domain.TopicTest, error) {
	const query = `SELECT test_index, COUNT(*), MAX(correct_count), MAX(total_questions), MAX(completed_at)
	               FROM practice_sessions
	               WHERE user_id = $1 AND pack_id = $2 AND category = $3
	                 AND test_index IS NOT NULL AND status = 'completed'
	               GROUP BY test_index`
	rows, err := r.db.Pool.Query(ctx, query, userID, packID, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make(map[int]domain.TopicTest)
	for rows.Next() {
		var test domain.TopicTest
		var completedAt *time.Time
		if err := rows.Scan(&test.Index, &test.Attempts, &test.BestCorrect, &test.QuestionCount, &completedAt); err != nil {
			return nil, err
		}
		test.LastAttemptAt = completedAt
		results[test.Index] = test
	}
	return results, rows.Err()
}

// CountCompletedTestsByCategory counts distinct tests, not attempts: three goes
// at Test 1 is one test covered.
func (r *PracticeRepository) CountCompletedTestsByCategory(ctx context.Context, userID int64, packID string) (map[string]int, error) {
	const query = `SELECT category, COUNT(DISTINCT test_index)
	               FROM practice_sessions
	               WHERE user_id = $1 AND pack_id = $2
	                 AND test_index IS NOT NULL AND status = 'completed' AND category IS NOT NULL
	               GROUP BY category`
	rows, err := r.db.Pool.Query(ctx, query, userID, packID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var category string
		var count int
		if err := rows.Scan(&category, &count); err != nil {
			return nil, err
		}
		counts[category] = count
	}
	return counts, rows.Err()
}

func (r *PracticeRepository) GetSessionTally(ctx context.Context, sessionID int64) (int, int, error) {
	query := `SELECT COUNT(*) FILTER (WHERE is_correct IS NOT NULL),
	                 COUNT(*) FILTER (WHERE is_correct)
	          FROM practice_session_questions WHERE session_id = $1`
	var answered, correct int
	if err := r.db.Pool.QueryRow(ctx, query, sessionID).Scan(&answered, &correct); err != nil {
		return 0, 0, err
	}
	return answered, correct, nil
}

func (r *PracticeRepository) CompleteSession(ctx context.Context, sessionID int64, answeredCount, correctCount int, completedAt time.Time) error {
	query := `UPDATE practice_sessions
	          SET status = 'completed', answered_count = $1, correct_count = $2, completed_at = $3
	          WHERE id = $4`
	_, err := r.db.Pool.Exec(ctx, query, answeredCount, correctCount, completedAt, sessionID)
	return err
}
