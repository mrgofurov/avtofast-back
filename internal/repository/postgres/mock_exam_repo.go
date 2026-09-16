package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/jackc/pgx/v5"
)

type MockExamRepository struct {
	db          *DB
	contentRepo *ContentRepository
}

func NewMockExamRepository(db *DB, contentRepo *ContentRepository) *MockExamRepository {
	return &MockExamRepository{db: db, contentRepo: contentRepo}
}

func (r *MockExamRepository) CreateExam(ctx context.Context, exam *domain.MockExam) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	queryExam := `INSERT INTO mock_exams (public_id, user_id, pack_id, pack_version, locale, format, status, started_at, deadline_at, question_count, pass_correct_count)
	              VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	              RETURNING id`
	err = tx.QueryRow(ctx, queryExam,
		exam.PublicID, exam.UserID, exam.PackID, exam.PackVersion, exam.Locale,
		exam.Format, exam.Status, exam.StartedAt, exam.DeadlineAt, exam.QuestionCount, exam.PassCorrectCount,
	).Scan(&exam.ID)
	if err != nil {
		return err
	}

	queryQ := `INSERT INTO mock_exam_questions (exam_id, question_id, order_index) VALUES ($1, $2, $3)`
	for i, q := range exam.Questions {
		_, err := tx.Exec(ctx, queryQ, exam.ID, q.ID, i+1)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *MockExamRepository) GetExamByPublicID(ctx context.Context, publicID string) (*domain.MockExam, error) {
	query := `SELECT id, public_id, user_id, pack_id, pack_version, locale, format, status,
	                 started_at, deadline_at, question_count, pass_correct_count, correct_count,
	                 score_percent, passed, completed_at
	          FROM mock_exams WHERE public_id = $1`
	var e domain.MockExam
	err := r.db.Pool.QueryRow(ctx, query, publicID).Scan(
		&e.ID, &e.PublicID, &e.UserID, &e.PackID, &e.PackVersion, &e.Locale, &e.Format, &e.Status,
		&e.StartedAt, &e.DeadlineAt, &e.QuestionCount, &e.PassCorrectCount, &e.CorrectCount,
		&e.ScorePercent, &e.Passed, &e.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	// Fetch ordered questions
	qQuery := `SELECT q.public_id FROM mock_exam_questions meq
	           JOIN questions q ON q.id = meq.question_id
	           WHERE meq.exam_id = $1 ORDER BY meq.order_index ASC`
	rows, err := r.db.Pool.Query(ctx, qQuery, e.ID)
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
	qMap := make(map[string]*domain.Question)
	for _, q := range questions {
		qMap[q.PublicID] = q
	}
	for _, pid := range qIDs {
		if q, ok := qMap[pid]; ok {
			e.Questions = append(e.Questions, q)
		}
	}

	return &e, nil
}

func (r *MockExamRepository) SaveExamAnswer(ctx context.Context, examID int64, questionID int64, selectedChoiceID string, elapsedMs int) error {
	query := `UPDATE mock_exam_questions
	          SET selected_choice_id = $1, elapsed_ms = $2, answered_at = NOW()
	          WHERE exam_id = $3 AND question_id = $4`
	_, err := r.db.Pool.Exec(ctx, query, selectedChoiceID, elapsedMs, examID, questionID)
	return err
}

func (r *MockExamRepository) GetExamAnswers(ctx context.Context, examID int64) (map[int64]string, error) {
	query := `SELECT question_id, selected_choice_id FROM mock_exam_questions
	          WHERE exam_id = $1 AND selected_choice_id IS NOT NULL`
	rows, err := r.db.Pool.Query(ctx, query, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	answers := map[int64]string{}
	for rows.Next() {
		var questionID int64
		var choice string
		if err := rows.Scan(&questionID, &choice); err != nil {
			return nil, err
		}
		answers[questionID] = choice
	}
	return answers, rows.Err()
}

func (r *MockExamRepository) SetExamAnswerCorrectness(ctx context.Context, examID int64, questionID int64, isCorrect bool) error {
	query := `UPDATE mock_exam_questions SET is_correct = $1 WHERE exam_id = $2 AND question_id = $3`
	_, err := r.db.Pool.Exec(ctx, query, isCorrect, examID, questionID)
	return err
}

func (r *MockExamRepository) CompleteExam(ctx context.Context, examID int64, correctCount, scorePercent int, passed bool, completedAt time.Time) error {
	query := `UPDATE mock_exams
	          SET status = 'completed', correct_count = $1, score_percent = $2, passed = $3, completed_at = $4
	          WHERE id = $5`
	_, err := r.db.Pool.Exec(ctx, query, correctCount, scorePercent, passed, completedAt, examID)
	return err
}

func (r *MockExamRepository) GetUserExams(ctx context.Context, userID int64, cursor string, limit int) ([]*domain.MockExam, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	whereClause := "WHERE user_id = $1"
	args := []any{userID}
	argIdx := 2

	if cursor != "" {
		whereClause += " AND public_id < $2"
		args = append(args, cursor)
		argIdx++
	}

	query := `SELECT id, public_id, user_id, pack_id, pack_version, locale, format, status,
	                 started_at, deadline_at, question_count, pass_correct_count, correct_count,
	                 score_percent, passed, completed_at
	          FROM mock_exams ` + whereClause + ` ORDER BY started_at DESC LIMIT ` + `$` + string(rune('0'+argIdx))
	args = append(args, limit+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var exams []*domain.MockExam
	for rows.Next() {
		var e domain.MockExam
		if err := rows.Scan(
			&e.ID, &e.PublicID, &e.UserID, &e.PackID, &e.PackVersion, &e.Locale, &e.Format, &e.Status,
			&e.StartedAt, &e.DeadlineAt, &e.QuestionCount, &e.PassCorrectCount, &e.CorrectCount,
			&e.ScorePercent, &e.Passed, &e.CompletedAt,
		); err != nil {
			return nil, "", err
		}
		exams = append(exams, &e)
	}

	var nextCursor string
	if len(exams) > limit {
		nextCursor = exams[limit-1].PublicID
		exams = exams[:limit]
	}

	return exams, nextCursor, nil
}
