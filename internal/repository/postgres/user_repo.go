package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/id"
	"github.com/jackc/pgx/v5"
)

type UserRepository struct {
	db *DB
}

func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	query := `SELECT id, public_id, provider_id, provider, email, phone, display_name, avatar_url, role, created_at, updated_at
	          FROM users WHERE id = $1`
	var u domain.User
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.PublicID, &u.ProviderID, &u.Provider, &u.Email, &u.Phone,
		&u.DisplayName, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetByPublicID(ctx context.Context, publicID string) (*domain.User, error) {
	query := `SELECT id, public_id, provider_id, provider, email, phone, display_name, avatar_url, role, created_at, updated_at
	          FROM users WHERE public_id = $1`
	var u domain.User
	err := r.db.Pool.QueryRow(ctx, query, publicID).Scan(
		&u.ID, &u.PublicID, &u.ProviderID, &u.Provider, &u.Email, &u.Phone,
		&u.DisplayName, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) GetOrCreateByProvider(ctx context.Context, provider, providerID, email, phone, displayName string) (*domain.User, error) {
	querySelect := `SELECT id, public_id, provider_id, provider, email, phone, display_name, avatar_url, role, created_at, updated_at
	                FROM users WHERE provider = $1 AND provider_id = $2`
	var u domain.User
	err := r.db.Pool.QueryRow(ctx, querySelect, provider, providerID).Scan(
		&u.ID, &u.PublicID, &u.ProviderID, &u.Provider, &u.Email, &u.Phone,
		&u.DisplayName, &u.AvatarURL, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == nil {
		return &u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Insert new user
	publicID := id.New(id.PrefixUser)
	now := time.Now().UTC()
	queryInsert := `INSERT INTO users (public_id, provider_id, provider, email, phone, display_name, role, created_at, updated_at)
	                VALUES ($1, $2, $3, $4, $5, $6, 'user', $7, $7)
	                RETURNING id`
	err = r.db.Pool.QueryRow(ctx, queryInsert, publicID, providerID, provider, email, phone, displayName, now).Scan(&u.ID)
	if err != nil {
		return nil, err
	}

	u.PublicID = publicID
	u.ProviderID = providerID
	u.Provider = provider
	u.Email = email
	u.Phone = phone
	u.DisplayName = displayName
	u.Role = domain.RoleUser
	u.CreatedAt = now
	u.UpdatedAt = now

	// Initialize default user stats
	_, _ = r.db.Pool.Exec(ctx, `INSERT INTO user_stats (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, u.ID)
	// Initialize default preferences
	_, _ = r.db.Pool.Exec(ctx, `INSERT INTO user_preferences (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, u.ID)
	// Initialize default notification preferences
	_, _ = r.db.Pool.Exec(ctx, `INSERT INTO notification_preferences (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, u.ID)
	// Initialize default free entitlement
	_, _ = r.db.Pool.Exec(ctx, `INSERT INTO entitlements (user_id, tier, status) VALUES ($1, 'free', 'active') ON CONFLICT DO NOTHING`, u.ID)

	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `UPDATE users SET display_name = $1, avatar_url = $2, role = $3, updated_at = NOW() WHERE id = $4`
	_, err := r.db.Pool.Exec(ctx, query, user.DisplayName, user.AvatarURL, user.Role, user.ID)
	return err
}

// DeleteUser removes the user row. Every user-scoped table declares
// `REFERENCES users(id) ON DELETE CASCADE`, so onboarding, preferences,
// devices, sessions, exams, mistakes, stats and purchases go with it in the
// same statement.
func (r *UserRepository) DeleteUser(ctx context.Context, userID int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return err
}

func (r *UserRepository) GetOnboarding(ctx context.Context, userID int64) (*domain.UserOnboarding, error) {
	query := `SELECT id, user_id, acquisition_source, knowledge_level, locale, target_exam_date, daily_question_goal, completed
	          FROM user_onboardings WHERE user_id = $1`
	var o domain.UserOnboarding
	var examDate time.Time
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&o.ID, &o.UserID, &o.AcquisitionSource, &o.KnowledgeLevel, &o.Locale, &examDate, &o.DailyQuestionGoal, &o.Completed,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	o.TargetExamDate = examDate.Format("2006-01-02")
	return &o, nil
}

func (r *UserRepository) SaveOnboarding(ctx context.Context, onboarding *domain.UserOnboarding) error {
	query := `INSERT INTO user_onboardings (user_id, acquisition_source, knowledge_level, locale, target_exam_date, daily_question_goal, completed, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, true, NOW())
	          ON CONFLICT (user_id) DO UPDATE SET
	            acquisition_source = EXCLUDED.acquisition_source,
	            knowledge_level = EXCLUDED.knowledge_level,
	            locale = EXCLUDED.locale,
	            target_exam_date = EXCLUDED.target_exam_date,
	            daily_question_goal = EXCLUDED.daily_question_goal,
	            completed = true,
	            updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query,
		onboarding.UserID, onboarding.AcquisitionSource, onboarding.KnowledgeLevel, onboarding.Locale,
		onboarding.TargetExamDate, onboarding.DailyQuestionGoal,
	)
	return err
}

func (r *UserRepository) GetPreferences(ctx context.Context, userID int64) (*domain.UserPreferences, error) {
	query := `SELECT id, user_id, locale, theme, profile_visibility, leaderboard_visibility
	          FROM user_preferences WHERE user_id = $1`
	var p domain.UserPreferences
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.Locale, &p.Theme, &p.ProfileVisibility, &p.LeaderboardVisibility,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.UserPreferences{
				UserID:                userID,
				Locale:                domain.LocaleUzLatn,
				Theme:                 "system",
				ProfileVisibility:     "friends",
				LeaderboardVisibility: "friends",
			}, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *UserRepository) UpdatePreferences(ctx context.Context, prefs *domain.UserPreferences) error {
	query := `INSERT INTO user_preferences (user_id, locale, theme, profile_visibility, leaderboard_visibility, updated_at)
	          VALUES ($1, $2, $3, $4, $5, NOW())
	          ON CONFLICT (user_id) DO UPDATE SET
	            locale = EXCLUDED.locale,
	            theme = EXCLUDED.theme,
	            profile_visibility = EXCLUDED.profile_visibility,
	            leaderboard_visibility = EXCLUDED.leaderboard_visibility,
	            updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query, prefs.UserID, prefs.Locale, prefs.Theme, prefs.ProfileVisibility, prefs.LeaderboardVisibility)
	return err
}

func (r *UserRepository) GetNotificationPreferences(ctx context.Context, userID int64) (*domain.NotificationPreferences, error) {
	query := `SELECT id, user_id, study_reminder, streak_protection, mistake_review, weekly_summary, score_improvement, reminder_time, timezone
	          FROM notification_preferences WHERE user_id = $1`
	var np domain.NotificationPreferences
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&np.ID, &np.UserID, &np.StudyReminder, &np.StreakProtection, &np.MistakeReview, &np.WeeklySummary, &np.ScoreImprovement, &np.ReminderTime, &np.Timezone,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.NotificationPreferences{
				UserID:           userID,
				StudyReminder:    true,
				StreakProtection: true,
				MistakeReview:    true,
				WeeklySummary:    true,
				ScoreImprovement: true,
				ReminderTime:     "19:00",
				Timezone:         "Asia/Tashkent",
			}, nil
		}
		return nil, err
	}
	return &np, nil
}

func (r *UserRepository) UpdateNotificationPreferences(ctx context.Context, prefs *domain.NotificationPreferences) error {
	query := `INSERT INTO notification_preferences (user_id, study_reminder, streak_protection, mistake_review, weekly_summary, score_improvement, reminder_time, timezone, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
	          ON CONFLICT (user_id) DO UPDATE SET
	            study_reminder = EXCLUDED.study_reminder,
	            streak_protection = EXCLUDED.streak_protection,
	            mistake_review = EXCLUDED.mistake_review,
	            weekly_summary = EXCLUDED.weekly_summary,
	            score_improvement = EXCLUDED.score_improvement,
	            reminder_time = EXCLUDED.reminder_time,
	            timezone = EXCLUDED.timezone,
	            updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query, prefs.UserID, prefs.StudyReminder, prefs.StreakProtection, prefs.MistakeReview, prefs.WeeklySummary, prefs.ScoreImprovement, prefs.ReminderTime, prefs.Timezone)
	return err
}
