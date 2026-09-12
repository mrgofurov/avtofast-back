package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

type MistakeRepository struct {
	db          *DB
	contentRepo *ContentRepository
}

func NewMistakeRepository(db *DB, contentRepo *ContentRepository) *MistakeRepository {
	return &MistakeRepository{db: db, contentRepo: contentRepo}
}

func (r *MistakeRepository) UpsertMistake(ctx context.Context, userID int64, questionID int64, isCorrect bool) error {
	now := time.Now().UTC()

	if !isCorrect {
		// Incorrect answer: increment mistake_count, reset consecutive_correct, set due soon (e.g. 1 day), high priority
		nextReview := now.Add(24 * time.Hour)
		query := `INSERT INTO user_mistakes (user_id, question_id, mistake_count, consecutive_correct, last_incorrect_at, next_review_at, priority, updated_at)
		          VALUES ($1, $2, 1, 0, $3, $4, 'high', $3)
		          ON CONFLICT (user_id, question_id) DO UPDATE SET
		            mistake_count = user_mistakes.mistake_count + 1,
		            consecutive_correct = 0,
		            last_incorrect_at = EXCLUDED.last_incorrect_at,
		            next_review_at = EXCLUDED.next_review_at,
		            priority = 'high',
		            updated_at = EXCLUDED.updated_at`
		_, err := r.db.Pool.Exec(ctx, query, userID, questionID, now, nextReview)
		return err
	}

	// Correct answer: advance spaced repetition interval
	// 1 correct: +3 days, 2 correct: +7 days, 3+: +14 days
	queryUpdate := `UPDATE user_mistakes
	                SET consecutive_correct = consecutive_correct + 1,
	                    next_review_at = CASE
	                      WHEN consecutive_correct = 0 THEN NOW() + INTERVAL '3 days'
	                      WHEN consecutive_correct = 1 THEN NOW() + INTERVAL '7 days'
	                      ELSE NOW() + INTERVAL '14 days'
	                    END,
	                    priority = CASE
	                      WHEN consecutive_correct = 0 THEN 'medium'
	                      ELSE 'low'
	                    END,
	                    updated_at = NOW()
	                WHERE user_id = $1 AND question_id = $2`
	_, err := r.db.Pool.Exec(ctx, queryUpdate, userID, questionID)
	return err
}

func (r *MistakeRepository) GetDueMistakes(ctx context.Context, userID int64, dueOnly bool, category string, cursor string, limit int) ([]*domain.UserMistakeItem, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	whereClause := "WHERE um.user_id = $1"
	args := []any{userID}
	argIdx := 2

	if dueOnly {
		whereClause += " AND um.next_review_at <= NOW()"
	}

	if category != "" {
		whereClause += fmt.Sprintf(" AND q.category = $%d", argIdx)
		args = append(args, category)
		argIdx++
	}

	if cursor != "" {
		whereClause += fmt.Sprintf(" AND q.public_id > $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query := fmt.Sprintf(`SELECT q.public_id, um.mistake_count, um.last_incorrect_at, um.next_review_at, um.priority
	                      FROM user_mistakes um
	                      JOIN questions q ON q.id = um.question_id
	                      %s
	                      ORDER BY um.priority DESC, um.next_review_at ASC, q.public_id ASC
	                      LIMIT $%d`, whereClause, argIdx)
	args = append(args, limit+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	type rawMistake struct {
		qPublicID string
		count     int
		lastInc   time.Time
		nextRev   time.Time
		priority  string
	}
	var rawList []rawMistake
	var qIDs []string

	for rows.Next() {
		var rm rawMistake
		if err := rows.Scan(&rm.qPublicID, &rm.count, &rm.lastInc, &rm.nextRev, &rm.priority); err != nil {
			return nil, "", err
		}
		rawList = append(rawList, rm)
		qIDs = append(qIDs, rm.qPublicID)
	}

	var nextCursor string
	if len(rawList) > limit {
		nextCursor = rawList[limit-1].qPublicID
		rawList = rawList[:limit]
		qIDs = qIDs[:limit]
	}

	questions, err := r.contentRepo.GetQuestionsByIDs(ctx, qIDs)
	if err != nil {
		return nil, "", err
	}

	qMap := make(map[string]*domain.Question)
	for _, q := range questions {
		qMap[q.PublicID] = q
	}

	var items []*domain.UserMistakeItem
	for _, rm := range rawList {
		q, ok := qMap[rm.qPublicID]
		if !ok {
			continue
		}
		items = append(items, &domain.UserMistakeItem{
			Question:        q.ToClientPayload(domain.LocaleUzLatn),
			MistakeCount:    rm.count,
			LastIncorrectAt: rm.lastInc,
			NextReviewAt:    rm.nextRev,
			Priority:        rm.priority,
		})
	}

	return items, nextCursor, nil
}
