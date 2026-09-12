package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/jackc/pgx/v5"
)

type DashboardRepository struct {
	db *DB
}

func NewDashboardRepository(db *DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

func (r *DashboardRepository) GetUserStats(ctx context.Context, userID int64) (*domain.UserStats, error) {
	query := `SELECT id, user_id, xp, level, streak_days, last_activity_date, total_answered, total_correct, completed_mock_exams, passed_mock_exams, updated_at
	          FROM user_stats WHERE user_id = $1`
	var s domain.UserStats
	var lastAct *time.Time
	err := r.db.Pool.QueryRow(ctx, query, userID).Scan(
		&s.ID, &s.UserID, &s.XP, &s.Level, &s.StreakDays, &lastAct,
		&s.TotalAnswered, &s.TotalCorrect, &s.CompletedMockExams, &s.PassedMockExams, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &domain.UserStats{
				UserID:     userID,
				XP:         0,
				Level:      1,
				StreakDays: 0,
			}, nil
		}
		return nil, err
	}
	if lastAct != nil {
		str := lastAct.Format("2006-01-02")
		s.LastActivityDate = &str
	}
	return &s, nil
}

func (r *DashboardRepository) UpdateUserStats(ctx context.Context, stats *domain.UserStats) error {
	query := `INSERT INTO user_stats (user_id, xp, level, streak_days, last_activity_date, total_answered, total_correct, completed_mock_exams, passed_mock_exams, updated_at)
	          VALUES ($1, $2, $3, $4, CURRENT_DATE, $5, $6, $7, $8, NOW())
	          ON CONFLICT (user_id) DO UPDATE SET
	            xp = EXCLUDED.xp,
	            level = EXCLUDED.level,
	            streak_days = EXCLUDED.streak_days,
	            last_activity_date = CURRENT_DATE,
	            total_answered = EXCLUDED.total_answered,
	            total_correct = EXCLUDED.total_correct,
	            completed_mock_exams = EXCLUDED.completed_mock_exams,
	            passed_mock_exams = EXCLUDED.passed_mock_exams,
	            updated_at = NOW()`
	_, err := r.db.Pool.Exec(ctx, query,
		stats.UserID, stats.XP, stats.Level, stats.StreakDays,
		stats.TotalAnswered, stats.TotalCorrect, stats.CompletedMockExams, stats.PassedMockExams,
	)
	return err
}

func (r *DashboardRepository) GetCategoryAccuracy(ctx context.Context, userID int64) ([]domain.WeakCategoryInfo, error) {
	query := `SELECT q.category,
	                 COUNT(psq.id) as total_attempts,
	                 COUNT(CASE WHEN psq.is_correct THEN 1 END) as correct_attempts,
	                 COALESCE(um.due_mistakes, 0) as due_mistakes
	          FROM practice_session_questions psq
	          JOIN practice_sessions ps ON ps.id = psq.session_id
	          JOIN questions q ON q.id = psq.question_id
	          LEFT JOIN (
	              SELECT q2.category, COUNT(um2.id) as due_mistakes
	              FROM user_mistakes um2
	              JOIN questions q2 ON q2.id = um2.question_id
	              WHERE um2.user_id = $1 AND um2.next_review_at <= NOW()
	              GROUP BY q2.category
	          ) um ON um.category = q.category
	          WHERE ps.user_id = $1 AND psq.is_correct IS NOT NULL
	          GROUP BY q.category, um.due_mistakes
	          ORDER BY (COUNT(CASE WHEN psq.is_correct THEN 1 END) * 100 / NULLIF(COUNT(psq.id), 0)) ASC
	          LIMIT 5`
	rows, err := r.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.WeakCategoryInfo
	for rows.Next() {
		var cat string
		var total, correct, due int
		if err := rows.Scan(&cat, &total, &correct, &due); err != nil {
			return nil, err
		}
		acc := 0
		if total > 0 {
			acc = (correct * 100) / total
		}
		list = append(list, domain.WeakCategoryInfo{
			Category:        cat,
			AccuracyPercent: acc,
			DueMistakes:     due,
		})
	}
	return list, nil
}

func (r *DashboardRepository) GetAnalyticsProgress(ctx context.Context, userID int64, days int) (map[string]any, error) {
	if days <= 0 {
		days = 30
	}
	stats, err := r.GetUserStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	catAcc, err := r.GetCategoryAccuracy(ctx, userID)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"rangeDays":            days,
		"overallAccuracy":      func() int { if stats.TotalAnswered == 0 { return 0 }; return (stats.TotalCorrect * 100) / stats.TotalAnswered }(),
		"totalQuestionsAnswered": stats.TotalAnswered,
		"completedMockExams":   stats.CompletedMockExams,
		"passedMockExams":      stats.PassedMockExams,
		"categories":           catAcc,
		"streak":               stats.StreakDays,
	}, nil
}
