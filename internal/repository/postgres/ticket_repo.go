package postgres

import (
	"context"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

// TicketRepository serves exam tickets: the list with the learner's scores,
// one ticket's questions in order, and the import that fills them.
type TicketRepository struct {
	db          *DB
	contentRepo *ContentRepository
}

func NewTicketRepository(db *DB, contentRepo *ContentRepository) *TicketRepository {
	return &TicketRepository{db: db, contentRepo: contentRepo}
}

func (r *TicketRepository) ListTickets(ctx context.Context, userID int64, packID string) ([]domain.Ticket, error) {
	// Scores come only from completed sessions: an abandoned attempt says
	// nothing about how well the ticket is known.
	const query = `SELECT t.number,
	                      (SELECT COUNT(*) FROM ticket_questions tq WHERE tq.ticket_id = t.id),
	                      COUNT(ps.id),
	                      MAX(ps.correct_count),
	                      MAX(ps.completed_at)
	               FROM tickets t
	               LEFT JOIN practice_sessions ps
	                      ON ps.pack_id = t.pack_id AND ps.ticket_number = t.number
	                     AND ps.user_id = $1 AND ps.status = 'completed'
	               WHERE t.pack_id = $2
	               GROUP BY t.id, t.number
	               ORDER BY t.number`
	rows, err := r.db.Pool.Query(ctx, query, userID, packID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Ticket
	for rows.Next() {
		var t domain.Ticket
		var best *int
		var last *time.Time
		if err := rows.Scan(&t.Number, &t.QuestionCount, &t.Attempts, &best, &last); err != nil {
			return nil, err
		}
		if t.Attempts > 0 {
			t.BestCorrect = best
			t.LastAttemptAt = last
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *TicketRepository) GetTicketQuestions(ctx context.Context, packID string, number int) ([]*domain.Question, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT q.public_id
	                                  FROM tickets t
	                                  JOIN ticket_questions tq ON tq.ticket_id = t.id
	                                  JOIN questions q ON q.id = tq.question_id
	                                  WHERE t.pack_id = $1 AND t.number = $2 AND q.status = 'published'
	                                  ORDER BY tq.position`, packID, number)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(ids) == 0 {
		return nil, err
	}

	// GetQuestionsByIDs loads translations but not in any order.
	loaded, err := r.contentRepo.GetQuestionsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*domain.Question, len(loaded))
	for _, q := range loaded {
		byID[q.PublicID] = q
	}
	ordered := make([]*domain.Question, 0, len(ids))
	for _, id := range ids {
		if q, ok := byID[id]; ok {
			ordered = append(ordered, q)
		}
	}
	return ordered, nil
}

// MarkContentDuplicates finds published questions that are word for word the
// same question under another id — same prompt, choices, picture, video and
// answer — keeps the oldest of each set and marks the rest "duplicate", so a
// topic never asks the same question twice. It returns duplicate → kept
// public ids, for tickets to point at the question that stays.
func (r *TicketRepository) MarkContentDuplicates(ctx context.Context, locale string) (map[string]string, error) {
	rows, err := r.db.Pool.Query(ctx, `
		WITH t AS (
			SELECT q.id, q.public_id, q.status,
			       FIRST_VALUE(q.public_id) OVER w AS kept
			FROM questions q
			JOIN question_translations tr ON tr.question_id = q.id AND tr.locale = $1
			WHERE q.status IN ('published', 'duplicate')
			WINDOW w AS (PARTITION BY tr.prompt, tr.choices::text, COALESCE(q.image_sha256, q.image_url, ''),
			                          COALESCE(q.video_url, ''), q.correct_choice_id
			             ORDER BY q.id)
		)
		SELECT public_id, kept FROM t WHERE public_id <> kept`, locale)
	if err != nil {
		return nil, err
	}
	dups := map[string]string{}
	for rows.Next() {
		var dup, kept string
		if err := rows.Scan(&dup, &kept); err != nil {
			rows.Close()
			return nil, err
		}
		dups[dup] = kept
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for dup := range dups {
		if _, err := r.db.Pool.Exec(ctx, `UPDATE questions SET status = 'duplicate', updated_at = NOW()
		                                  WHERE public_id = $1 AND status <> 'duplicate'`, dup); err != nil {
			return nil, err
		}
	}
	return dups, nil
}

// TicketImport is one ticket to store: its number and its questions' public
// ids, in order.
type TicketImport struct {
	Number      int
	Source      string
	QuestionIDs []string
}

// ReplaceTickets stores the tickets given, replacing the question list of any
// that already exist, all or none. Questions not in the bank are reported
// back and left out of their ticket.
func (r *TicketRepository) ReplaceTickets(ctx context.Context, packID string, tickets []TicketImport) (missing []string, err error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, t := range tickets {
		var ticketID int64
		err := tx.QueryRow(ctx, `INSERT INTO tickets (pack_id, number, source)
		                         VALUES ($1, $2, $3)
		                         ON CONFLICT (pack_id, number) DO UPDATE SET source = EXCLUDED.source, updated_at = NOW()
		                         RETURNING id`, packID, t.Number, t.Source).Scan(&ticketID)
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM ticket_questions WHERE ticket_id = $1`, ticketID); err != nil {
			return nil, err
		}
		position := 0
		for _, publicID := range t.QuestionIDs {
			var questionID int64
			err := tx.QueryRow(ctx, `SELECT id FROM questions WHERE public_id = $1`, publicID).Scan(&questionID)
			if err != nil {
				missing = append(missing, publicID)
				continue
			}
			position++
			if _, err := tx.Exec(ctx, `INSERT INTO ticket_questions (ticket_id, position, question_id) VALUES ($1, $2, $3)`,
				ticketID, position, questionID); err != nil {
				return nil, err
			}
		}
	}
	return missing, tx.Commit(ctx)
}
