package postgres

import (
	"context"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/jackc/pgx/v5"
)

type LegalRepository struct {
	db *DB
}

func NewLegalRepository(db *DB) *LegalRepository {
	return &LegalRepository{db: db}
}

func (r *LegalRepository) ListParagraphs(ctx context.Context) ([]domain.LegalParagraph, error) {
	rows, err := r.db.Pool.Query(ctx, `SELECT document, document_title, source_element_id, source_url, ordinal,
	                                         chapter, appendix, section, clause_number, text
	                                  FROM legal_paragraphs ORDER BY document, ordinal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.LegalParagraph
	for rows.Next() {
		var p domain.LegalParagraph
		if err := rows.Scan(&p.Document, &p.DocumentTitle, &p.SourceElementID, &p.SourceURL, &p.Ordinal,
			&p.Chapter, &p.Appendix, &p.Section, &p.ClauseNumber, &p.Text); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *LegalRepository) ReplaceDocument(ctx context.Context, document string, paragraphs []domain.LegalParagraph) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM legal_paragraphs WHERE document = $1`, document); err != nil {
		return err
	}
	rows := make([][]any, len(paragraphs))
	for i, p := range paragraphs {
		rows[i] = []any{document, p.DocumentTitle, p.SourceElementID, p.SourceURL, p.Ordinal,
			p.Chapter, p.Appendix, p.Section, p.ClauseNumber, p.Text}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"legal_paragraphs"},
		[]string{"document", "document_title", "source_element_id", "source_url", "ordinal",
			"chapter", "appendix", "section", "clause_number", "text"},
		pgx.CopyFromRows(rows)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
