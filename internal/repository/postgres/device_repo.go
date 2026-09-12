package postgres

import (
	"context"
	"errors"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/jackc/pgx/v5"
)

type DeviceRepository struct {
	db *DB
}

func NewDeviceRepository(db *DB) *DeviceRepository {
	return &DeviceRepository{db: db}
}

func (r *DeviceRepository) UpsertDevice(ctx context.Context, device *domain.Device) error {
	query := `INSERT INTO devices (public_id, user_id, platform, push_token, app_version, locale, timezone, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	          ON CONFLICT (public_id) DO UPDATE SET
	            user_id = EXCLUDED.user_id,
	            platform = EXCLUDED.platform,
	            push_token = EXCLUDED.push_token,
	            app_version = EXCLUDED.app_version,
	            locale = EXCLUDED.locale,
	            timezone = EXCLUDED.timezone,
	            updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query,
		device.PublicID, device.UserID, device.Platform, device.PushToken,
		device.AppVersion, device.Locale, device.Timezone,
	)
	return err
}

func (r *DeviceRepository) GetDeviceByPublicID(ctx context.Context, publicID string) (*domain.Device, error) {
	query := `SELECT id, public_id, user_id, platform, push_token, app_version, locale, timezone
	          FROM devices WHERE public_id = $1`
	var d domain.Device
	err := r.db.Pool.QueryRow(ctx, query, publicID).Scan(
		&d.ID, &d.PublicID, &d.UserID, &d.Platform, &d.PushToken, &d.AppVersion, &d.Locale, &d.Timezone,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &d, nil
}

func (r *DeviceRepository) DeleteDeviceToken(ctx context.Context, publicID string) error {
	query := `UPDATE devices SET push_token = NULL, updated_at = NOW() WHERE public_id = $1`
	_, err := r.db.Pool.Exec(ctx, query, publicID)
	return err
}
