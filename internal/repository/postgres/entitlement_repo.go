package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/id"
	"github.com/jackc/pgx/v5"
)

type EntitlementRepository struct {
	db *DB
}

func NewEntitlementRepository(db *DB) *EntitlementRepository {
	return &EntitlementRepository{db: db}
}

func (r *EntitlementRepository) GetByUserID(ctx context.Context, userID int64) (*domain.Entitlement, error) {
	query := `SELECT id, user_id, tier, status, product_id, expires_at, features
	          FROM entitlements WHERE user_id = $1`
	var e domain.Entitlement
	var featRaw []byte
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&e.ID, &e.UserID, &e.Tier, &e.Status, &e.ProductID, &e.ExpiresAt, &featRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.Entitlement{
				UserID:   userID,
				Tier:     domain.TierFree,
				Status:   "active",
				Features: map[string]bool{
					"aiMistakeAnalysis": false,
					"advancedAnalytics": false,
					"voiceExplanations": false,
					"premiumMockExams":  false,
					"ads":               true,
				},
			}, nil
		}
		return nil, err
	}

	_ = json.Unmarshal(featRaw, &e.Features)
	if e.Features == nil {
		e.Features = make(map[string]bool)
	}

	// Check expiration
	if e.ExpiresAt != nil && time.Now().UTC().After(*e.ExpiresAt) {
		e.Tier = domain.TierFree
		e.Status = "expired"
		e.Features["aiMistakeAnalysis"] = false
		e.Features["advancedAnalytics"] = false
		e.Features["voiceExplanations"] = false
		e.Features["premiumMockExams"] = false
		e.Features["ads"] = true
	}

	return &e, nil
}

func (r *EntitlementRepository) SaveEntitlement(ctx context.Context, ent *domain.Entitlement) error {
	featRaw, _ := json.Marshal(ent.Features)
	query := `INSERT INTO entitlements (user_id, tier, status, product_id, expires_at, features, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, NOW())
	          ON CONFLICT (user_id) DO UPDATE SET
	            tier = EXCLUDED.tier,
	            status = EXCLUDED.status,
	            product_id = EXCLUDED.product_id,
	            expires_at = EXCLUDED.expires_at,
	            features = EXCLUDED.features,
	            updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query, ent.UserID, ent.Tier, ent.Status, ent.ProductID, ent.ExpiresAt, featRaw)
	return err
}

func (r *EntitlementRepository) RecordPurchase(ctx context.Context, userID int64, platform, transactionID, productID, rawPayload string) error {
	pubID := id.New(id.PrefixPurchase)
	query := `INSERT INTO purchases (public_id, user_id, platform, transaction_id, product_id, raw_payload, verified_at)
	          VALUES ($1, $2, $3, $4, $5, $6, NOW())
	          ON CONFLICT (transaction_id) DO NOTHING`
	_, err := r.db.Pool.Exec(ctx, query, pubID, userID, platform, transactionID, productID, rawPayload)
	return err
}
