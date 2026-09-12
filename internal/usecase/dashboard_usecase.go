package usecase

import (
	"context"

	"github.com/avtofast/avtofast-back/internal/domain"
)

type DashboardUsecase struct {
	dashRepo    domain.DashboardRepository
	mistakeRepo domain.MistakeRepository
	userRepo    domain.UserRepository
}

func NewDashboardUsecase(dashRepo domain.DashboardRepository, mistakeRepo domain.MistakeRepository, userRepo domain.UserRepository) *DashboardUsecase {
	return &DashboardUsecase{
		dashRepo:    dashRepo,
		mistakeRepo: mistakeRepo,
		userRepo:    userRepo,
	}
}

func (u *DashboardUsecase) GetDashboard(ctx context.Context, userID int64) (*domain.DashboardSnapshot, error) {
	stats, err := u.dashRepo.GetUserStats(ctx, userID)
	if err != nil {
		return nil, err
	}

	onboarding, _ := u.userRepo.GetOnboarding(ctx, userID)
	targetDaily := 10
	if onboarding != nil && onboarding.DailyQuestionGoal > 0 {
		targetDaily = onboarding.DailyQuestionGoal
	}

	dueMistakes, _, _ := u.mistakeRepo.GetDueMistakes(ctx, userID, true, "", "", 10)
	weakCats, _ := u.dashRepo.GetCategoryAccuracy(ctx, userID)

	acc := 0
	if stats.TotalAnswered > 0 {
		acc = (stats.TotalCorrect * 100) / stats.TotalAnswered
	}

	// Readiness score formula (0-100)
	readiness := 60
	if stats.CompletedMockExams > 0 {
		readiness = 70 + (stats.PassedMockExams * 5)
		if readiness > 95 {
			readiness = 95
		}
	}

	nextAction := "daily_goal"
	if len(dueMistakes) > 0 {
		nextAction = "mistake_review"
	} else if stats.CompletedMockExams == 0 {
		nextAction = "mock_exam"
	}

	return &domain.DashboardSnapshot{
		XP:                 stats.XP,
		Level:              stats.Level,
		StreakDays:         stats.StreakDays,
		DailyGoal:          domain.DailyGoalStatus{Completed: stats.TotalAnswered % targetDaily, Target: targetDaily},
		ReadinessScore:     readiness,
		AccuracyPercent:    acc,
		CompletedMockExams: stats.CompletedMockExams,
		WeakCategories:     weakCats,
		NextRecommendedAction: nextAction,
	}, nil
}

func (u *DashboardUsecase) GetAnalyticsProgress(ctx context.Context, userID int64, days int) (map[string]any, error) {
	return u.dashRepo.GetAnalyticsProgress(ctx, userID, days)
}

func (u *DashboardUsecase) GetMistakes(ctx context.Context, userID int64, dueOnly bool, category, cursor string, limit int) ([]*domain.UserMistakeItem, string, error) {
	return u.mistakeRepo.GetDueMistakes(ctx, userID, dueOnly, category, cursor, limit)
}
