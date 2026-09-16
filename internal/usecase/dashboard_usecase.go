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

	// The review queue is a headline number on two screens, so it is counted
	// in full rather than capped at a page of results.
	dueMistakes, _, _ := u.mistakeRepo.GetDueMistakes(ctx, userID, true, "", "", 100)
	weakCats, _ := u.dashRepo.GetCategoryAccuracy(ctx, userID)

	acc := 0
	if stats.TotalAnswered > 0 {
		acc = (stats.TotalCorrect * 100) / stats.TotalAnswered
	}

	// Today's answers, not a running total modulo the goal: the old formula
	// reset the ring at arbitrary moments and could show progress on a day
	// the learner had not opened the app.
	completedToday := 0
	if daily, err := u.dashRepo.GetDailyActivity(ctx, userID, 1); err == nil && len(daily) > 0 {
		completedToday = daily[len(daily)-1].Answered
	}
	if completedToday > targetDaily {
		completedToday = targetDaily
	}

	nextAction := "daily_goal"
	if len(dueMistakes) > 0 {
		nextAction = "mistake_review"
	} else if stats.CompletedMockExams == 0 {
		nextAction = "mock_exam"
	}

	return &domain.DashboardSnapshot{
		XP:                    stats.XP,
		Level:                 stats.Level,
		StreakDays:            stats.StreakDays,
		DailyGoal:             domain.DailyGoalStatus{Completed: completedToday, Target: targetDaily},
		ReadinessScore:        readinessScore(stats, acc),
		AccuracyPercent:       acc,
		CompletedMockExams:    stats.CompletedMockExams,
		DueMistakeCount:       len(dueMistakes),
		WeakCategories:        weakCats,
		NextRecommendedAction: nextAction,
	}, nil
}

// readinessScore answers the one question the home screen exists to answer:
// would this learner pass the exam today?
//
// Three things decide it, in the order they matter. Accuracy is the best
// single predictor, so it carries the most weight. Practice volume is what
// makes that accuracy trustworthy — 90% over twenty questions says much less
// than 90% over four hundred — and it saturates at 500 answers, roughly a full
// pass of the question bank. Mock exams are the closest thing to the real
// test, so passing them is worth its own share. A learner who has answered
// nothing scores zero rather than a flattering default.
func readinessScore(stats *domain.UserStats, accuracyPercent int) int {
	if stats.TotalAnswered == 0 {
		return 0
	}

	const (
		accuracyWeight = 60
		volumeWeight   = 20
		examWeight     = 20
		volumeTarget   = 500
		examTarget     = 3
	)

	volume := stats.TotalAnswered
	if volume > volumeTarget {
		volume = volumeTarget
	}
	exams := stats.PassedMockExams
	if exams > examTarget {
		exams = examTarget
	}

	score := (accuracyPercent*accuracyWeight)/100 +
		(volume*volumeWeight)/volumeTarget +
		(exams*examWeight)/examTarget
	if score > 100 {
		return 100
	}
	return score
}

func (u *DashboardUsecase) GetAnalyticsProgress(ctx context.Context, userID int64, days int) (map[string]any, error) {
	return u.dashRepo.GetAnalyticsProgress(ctx, userID, days)
}

func (u *DashboardUsecase) GetMistakes(ctx context.Context, userID int64, dueOnly bool, category, cursor string, limit int) ([]*domain.UserMistakeItem, string, error) {
	return u.mistakeRepo.GetDueMistakes(ctx, userID, dueOnly, category, cursor, limit)
}
