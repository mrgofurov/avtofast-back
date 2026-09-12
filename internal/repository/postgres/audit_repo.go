package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/avtofast/avtofast-back/internal/domain"
)

type AuditRepository struct {
	db *DB
}

func NewAuditRepository(db *DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) LogAction(ctx context.Context, log *domain.AuditLog) error {
	beforeRaw, _ := json.Marshal(log.BeforeState)
	afterRaw, _ := json.Marshal(log.AfterState)

	query := `INSERT INTO admin_audit_logs (public_id, actor_id, action, target_type, target_id, before_state, after_state, reason, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`
	_, err := r.db.Pool.Exec(ctx, query,
		log.PublicID, log.ActorID, log.Action, log.TargetType, log.TargetID,
		beforeRaw, afterRaw, log.Reason,
	)
	return err
}

func (r *AuditRepository) GetAuditLogs(ctx context.Context, cursor string, limit int) ([]*domain.AuditLog, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	whereClause := ""
	args := []any{}
	if cursor != "" {
		whereClause = "WHERE a.public_id < $1"
		args = append(args, cursor)
	}

	query := fmt.Sprintf(`SELECT a.id, a.public_id, a.actor_id, COALESCE(u.display_name, 'Admin'),
	                             a.action, a.target_type, a.target_id, a.before_state, a.after_state, a.reason, a.created_at
	                      FROM admin_audit_logs a
	                      LEFT JOIN users u ON u.id = a.actor_id
	                      %s
	                      ORDER BY a.created_at DESC LIMIT $%d`, whereClause, len(args)+1)
	args = append(args, limit+1)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var logs []*domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		var beforeRaw, afterRaw []byte
		if err := rows.Scan(
			&l.ID, &l.PublicID, &l.ActorID, &l.ActorName, &l.Action, &l.TargetType,
			&l.TargetID, &beforeRaw, &afterRaw, &l.Reason, &l.CreatedAt,
		); err != nil {
			return nil, "", err
		}
		_ = json.Unmarshal(beforeRaw, &l.BeforeState)
		_ = json.Unmarshal(afterRaw, &l.AfterState)
		logs = append(logs, &l)
	}

	var nextCursor string
	if len(logs) > limit {
		nextCursor = logs[limit-1].PublicID
		logs = logs[:limit]
	}

	return logs, nextCursor, nil
}
