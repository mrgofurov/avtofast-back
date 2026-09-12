package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

type SyncRepository struct {
	db          *DB
	userRepo    *UserRepository
	entRepo     *EntitlementRepository
	dashRepo    *DashboardRepository
	mistakeRepo *MistakeRepository
}

func NewSyncRepository(db *DB, userRepo *UserRepository, entRepo *EntitlementRepository, dashRepo *DashboardRepository, mistakeRepo *MistakeRepository) *SyncRepository {
	return &SyncRepository{
		db:          db,
		userRepo:    userRepo,
		entRepo:     entRepo,
		dashRepo:    dashRepo,
		mistakeRepo: mistakeRepo,
	}
}

func (r *SyncRepository) SaveSyncEvent(ctx context.Context, event *domain.SyncEventItem, userID int64) error {
	payloadBytes, _ := json.Marshal(event.Payload)
	query := `INSERT INTO sync_events (public_id, user_id, event_type, occurred_at, pack_id, pack_version, payload, synced_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	          ON CONFLICT (public_id) DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, query,
		event.ID, userID, event.Type, event.OccurredAt, event.PackID, event.PackVersion, payloadBytes,
	)
	return err
}

func (r *SyncRepository) GetChangesSince(ctx context.Context, userID int64, cursor string) (*domain.SyncChangesResponse, error) {
	prefs, err := r.userRepo.GetPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}

	ent, err := r.entRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	dueMistakes, _, err := r.mistakeRepo.GetDueMistakes(ctx, userID, true, "", "", 100)
	if err != nil {
		return nil, err
	}

	stats, err := r.dashRepo.GetUserStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	newCursor := fmt.Sprintf("cur_%d", time.Now().UTC().UnixNano())

	return &domain.SyncChangesResponse{
		ProfileChanges:   prefs,
		Entitlement:      ent,
		ReviewQueueCount: len(dueMistakes),
		Dashboard:        stats,
		NextCursor:       newCursor,
	}, nil
}
