package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// QuestionText is what topic classification reads of a question.
type QuestionText struct {
	PublicID    string
	Category    string
	Prompt      string
	Explanation string
}

// TopicRepository serves cmd/classify-topics: every question's text in one
// locale, and a way to file questions under their topics.
type TopicRepository struct {
	db *DB
}

func NewTopicRepository(db *DB) *TopicRepository {
	return &TopicRepository{db: db}
}

func (r *TopicRepository) ListQuestionTexts(ctx context.Context, locale string) ([]QuestionText, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT q.public_id, q.category, COALESCE(t.prompt, ''), COALESCE(t.explanation, '')
	                                  FROM questions q
	                                  LEFT JOIN question_translations t ON t.question_id = q.id AND t.locale = $1
	                                  ORDER BY q.id`, locale)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []QuestionText
	for rows.Next() {
		var q QuestionText
		if err := rows.Scan(&q.PublicID, &q.Category, &q.Prompt, &q.Explanation); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// SetCategories files questions under topics (public id → topic key), all or
// none.
func (r *TopicRepository) SetCategories(ctx context.Context, topics map[string]string) error {
	batch := &pgx.Batch{}
	for id, topic := range topics {
		batch.Queue(`UPDATE questions SET category = $1, updated_at = NOW() WHERE public_id = $2 AND category <> $1`, topic, id)
	}
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := tx.SendBatch(ctx, batch).Close(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
