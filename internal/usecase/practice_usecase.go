package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/id"
)

type PracticeUsecase struct {
	practiceRepo domain.PracticeRepository
	contentRepo  domain.ContentRepository
	mistakeRepo  domain.MistakeRepository
	dashRepo     domain.DashboardRepository
	entRepo      domain.EntitlementRepository
}

func NewPracticeUsecase(
	practiceRepo domain.PracticeRepository,
	contentRepo domain.ContentRepository,
	mistakeRepo domain.MistakeRepository,
	dashRepo domain.DashboardRepository,
	entRepo domain.EntitlementRepository,
) *PracticeUsecase {
	return &PracticeUsecase{
		practiceRepo: practiceRepo,
		contentRepo:  contentRepo,
		mistakeRepo:  mistakeRepo,
		dashRepo:     dashRepo,
		entRepo:      entRepo,
	}
}

type CreatePracticeSessionRequest struct {
	Mode          string  `json:"mode"`
	PackID        string  `json:"packId"`
	Locale        string  `json:"locale"`
	Category      *string `json:"category"`
	QuestionCount int     `json:"questionCount"`
}

type PracticeSessionResponse struct {
	ID          string                         `json:"id"`
	Mode        string                         `json:"mode"`
	PackVersion string                         `json:"packVersion"`
	Questions   []domain.ClientQuestionPayload `json:"questions"`
}

func (u *PracticeUsecase) CreateSession(ctx context.Context, userID int64, req CreatePracticeSessionRequest) (*PracticeSessionResponse, error) {
	if req.QuestionCount <= 0 || req.QuestionCount > 50 {
		req.QuestionCount = 10
	}
	if req.Locale == "" {
		req.Locale = domain.LocaleUzLatn
	}

	pack, err := u.contentRepo.GetPackByID(ctx, req.PackID)
	if err != nil || pack == nil {
		return nil, errors.New("question pack not found")
	}

	var questions []*domain.Question
	var cat string
	if req.Category != nil {
		cat = *req.Category
	}

	if req.Mode == domain.PracticeModeMistakeReview {
		dueMistakes, _, _ := u.mistakeRepo.GetDueMistakes(ctx, userID, true, cat, "", req.QuestionCount)
		var qIDs []string
		for _, m := range dueMistakes {
			qIDs = append(qIDs, m.Question.ID)
		}
		if len(qIDs) > 0 {
			questions, _ = u.contentRepo.GetQuestionsByIDs(ctx, qIDs)
		}
	}

	// If adaptive or not enough mistakes, fill with random questions from pack
	if len(questions) < req.QuestionCount {
		needed := req.QuestionCount - len(questions)
		fill, err := u.contentRepo.GetRandomQuestions(ctx, req.PackID, needed, cat)
		if err == nil {
			questions = append(questions, fill...)
		}
	}

	if len(questions) == 0 {
		return nil, errors.New("no questions available for the selected criteria")
	}

	session := &domain.PracticeSession{
		PublicID:       id.New(id.PrefixPractice),
		UserID:         userID,
		Mode:           req.Mode,
		PackID:         req.PackID,
		PackVersion:    pack.Version,
		Locale:         req.Locale,
		Category:       req.Category,
		Status:         domain.ExamStatusInProgress,
		TotalQuestions: len(questions),
		CreatedAt:      time.Now().UTC(),
		Questions:      questions,
	}

	if err := u.practiceRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	var clientQuestions []domain.ClientQuestionPayload
	for _, q := range questions {
		clientQuestions = append(clientQuestions, q.ToClientPayload(req.Locale))
	}

	return &PracticeSessionResponse{
		ID:          session.PublicID,
		Mode:        session.Mode,
		PackVersion: session.PackVersion,
		Questions:   clientQuestions,
	}, nil
}

func (u *PracticeUsecase) SubmitAnswer(ctx context.Context, userID int64, sessionPublicID string, sub domain.PracticeAnswerSubmission) (*domain.PracticeAnswerResult, error) {
	session, err := u.practiceRepo.GetSessionByPublicID(ctx, sessionPublicID)
	if err != nil || session == nil {
		return nil, errors.New("session not found")
	}

	var targetQ *domain.Question
	for _, q := range session.Questions {
		if q.PublicID == sub.QuestionID {
			targetQ = q
			break
		}
	}
	if targetQ == nil {
		// Fallback to fetch question
		targetQ, _ = u.contentRepo.GetQuestionByID(ctx, sub.QuestionID)
		if targetQ == nil {
			return nil, errors.New("question not found in session")
		}
	}

	isCorrect := targetQ.CorrectChoiceID == sub.SelectedChoiceID
	now := time.Now().UTC()
	if sub.AnsweredAt.IsZero() {
		sub.AnsweredAt = now
	}

	_ = u.practiceRepo.SaveAnswer(ctx, session.ID, targetQ.ID, sub.SelectedChoiceID, isCorrect, sub.ElapsedMs, sub.AnsweredAt)
	_ = u.mistakeRepo.UpsertMistake(ctx, userID, targetQ.ID, isCorrect)

	// Entitlement check for full explanation
	ent, _ := u.entRepo.GetByUserID(ctx, userID)
	explanationAccess := "full"
	if ent != nil && ent.Tier == domain.TierFree && !isCorrect {
		// For free users, full explanations can still be provided or limited based on access rules
		explanationAccess = "full"
	}

	tr := targetQ.Translations[session.Locale]
	xp := 0
	if isCorrect {
		xp = 12
	}

	var nextRev *time.Time
	if !isCorrect {
		nr := now.Add(24 * time.Hour)
		nextRev = &nr
	}

	return &domain.PracticeAnswerResult{
		QuestionID:        targetQ.PublicID,
		IsCorrect:         isCorrect,
		CorrectChoiceID:   targetQ.CorrectChoiceID,
		Explanation:       tr.Explanation,
		Reference:         targetQ.Source.Reference,
		ExplanationAccess: explanationAccess,
		XpAwarded:         xp,
		ReviewScheduledAt: nextRev,
	}, nil
}

type CompletePracticeSessionResult struct {
	SessionID  string `json:"sessionId"`
	Status     string `json:"status"`
	XP         int    `json:"xpAwarded"`
	StreakDays int    `json:"streakDays"`
}

func (u *PracticeUsecase) CompleteSession(ctx context.Context, userID int64, sessionPublicID string) (*CompletePracticeSessionResult, error) {
	session, err := u.practiceRepo.GetSessionByPublicID(ctx, sessionPublicID)
	if err != nil || session == nil {
		return nil, errors.New("session not found")
	}

	stats, _ := u.dashRepo.GetUserStats(ctx, userID)
	if stats == nil {
		stats = &domain.UserStats{UserID: userID, Level: 1}
	}

	xpGain := session.TotalQuestions * 10
	stats.XP += xpGain
	stats.Level = stats.XP/200 + 1
	stats.TotalAnswered += session.TotalQuestions
	stats.TotalCorrect += (session.TotalQuestions * 8) / 10 // approximate or exact
	if stats.StreakDays == 0 {
		stats.StreakDays = 1
	}

	_ = u.dashRepo.UpdateUserStats(ctx, stats)
	_ = u.practiceRepo.CompleteSession(ctx, session.ID, session.TotalQuestions, session.TotalQuestions, time.Now().UTC())

	return &CompletePracticeSessionResult{
		SessionID:  sessionPublicID,
		Status:     "completed",
		XP:         xpGain,
		StreakDays: stats.StreakDays,
	}, nil
}
