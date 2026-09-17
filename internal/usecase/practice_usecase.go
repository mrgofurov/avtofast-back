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
	Mode     string  `json:"mode"`
	PackID   string  `json:"packId"`
	Locale   string  `json:"locale"`
	Category *string `json:"category"`
	// TestIndex asks for one numbered test of a category: a fixed slice of the
	// bank rather than a random draw, so Test 7 is the same twenty questions on
	// every attempt and on every device. 1-based. Ignored without a category.
	TestIndex     *int `json:"testIndex"`
	QuestionCount int  `json:"questionCount"`
}

type PracticeSessionResponse struct {
	ID          string                         `json:"id"`
	Mode        string                         `json:"mode"`
	PackVersion string                         `json:"packVersion"`
	TestIndex   *int                           `json:"testIndex,omitempty"`
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

	// A numbered test is answered entirely from the slice: no mistake-review
	// fill, no random top-up. Either the learner gets exactly that test or the
	// request fails, because a "Test 7" that quietly contained something else
	// would make its score meaningless.
	if testIndex := req.TestIndex; testIndex != nil {
		if cat == "" {
			return nil, errors.New("testIndex requires a category")
		}
		if *testIndex < 1 {
			return nil, errors.New("testIndex is 1-based")
		}
		slice, err := u.contentRepo.GetQuestionSlice(
			ctx, req.PackID, cat,
			(*testIndex-1)*domain.QuestionsPerTest, domain.QuestionsPerTest,
		)
		if err != nil {
			return nil, err
		}
		if len(slice) == 0 {
			return nil, errors.New("no such test in this category")
		}
		return u.persistSession(ctx, userID, req, pack, slice, testIndex)
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

	return u.persistSession(ctx, userID, req, pack, questions, nil)
}

func (u *PracticeUsecase) persistSession(
	ctx context.Context,
	userID int64,
	req CreatePracticeSessionRequest,
	pack *domain.QuestionPack,
	questions []*domain.Question,
	testIndex *int,
) (*PracticeSessionResponse, error) {
	session := &domain.PracticeSession{
		PublicID:       id.New(id.PrefixPractice),
		UserID:         userID,
		Mode:           req.Mode,
		PackID:         req.PackID,
		PackVersion:    pack.Version,
		Locale:         req.Locale,
		Category:       req.Category,
		TestIndex:      testIndex,
		Status:         domain.ExamStatusInProgress,
		TotalQuestions: len(questions),
		CreatedAt:      time.Now().UTC(),
		Questions:      questions,
	}

	if err := u.practiceRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	clientQuestions := make([]domain.ClientQuestionPayload, 0, len(questions))
	for _, q := range questions {
		clientQuestions = append(clientQuestions, q.ToClientPayload(req.Locale))
	}

	return &PracticeSessionResponse{
		ID:          session.PublicID,
		Mode:        session.Mode,
		PackVersion: session.PackVersion,
		TestIndex:   testIndex,
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
	xp := xpPerIncorrectAnswer
	if isCorrect {
		xp = xpPerCorrectAnswer
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
	SessionID     string `json:"sessionId"`
	Status        string `json:"status"`
	XP            int    `json:"xpAwarded"`
	StreakDays    int    `json:"streakDays"`
	AnsweredCount int    `json:"answeredCount"`
	CorrectCount  int    `json:"correctCount"`
}

// XP per answer. A wrong answer still pays, because the thing being rewarded
// is showing up; it pays less, because the thing being measured is knowing.
const (
	xpPerCorrectAnswer   = 12
	xpPerIncorrectAnswer = 2
)

func (u *PracticeUsecase) CompleteSession(ctx context.Context, userID int64, sessionPublicID string) (*CompletePracticeSessionResult, error) {
	session, err := u.practiceRepo.GetSessionByPublicID(ctx, sessionPublicID)
	if err != nil || session == nil {
		return nil, errors.New("session not found")
	}

	if session.Status == domain.ExamStatusCompleted {
		// Completing twice must not pay twice — the mobile client retries a
		// failed completion, and the idempotency key only covers a repeat of
		// the same request, not a second attempt with a fresh key.
		return &CompletePracticeSessionResult{
			SessionID:     sessionPublicID,
			Status:        "completed",
			AnsweredCount: session.AnsweredCount,
			CorrectCount:  session.CorrectCount,
		}, nil
	}

	// What the learner actually answered, not what they were handed: a
	// session abandoned after three of ten questions scores three.
	answered, correct, err := u.practiceRepo.GetSessionTally(ctx, session.ID)
	if err != nil {
		return nil, err
	}

	stats, _ := u.dashRepo.GetUserStats(ctx, userID)
	if stats == nil {
		stats = &domain.UserStats{UserID: userID, Level: 1}
	}

	xpGain := correct*xpPerCorrectAnswer + (answered-correct)*xpPerIncorrectAnswer
	stats.XP += xpGain
	stats.Level = stats.XP/200 + 1
	stats.TotalAnswered += answered
	stats.TotalCorrect += correct
	if answered > 0 {
		stats.StreakDays = nextStreak(stats.StreakDays, stats.LastActivityDate, time.Now().UTC())
	}

	_ = u.dashRepo.UpdateUserStats(ctx, stats)
	_ = u.practiceRepo.CompleteSession(ctx, session.ID, answered, correct, time.Now().UTC())

	return &CompletePracticeSessionResult{
		SessionID:     sessionPublicID,
		Status:        "completed",
		XP:            xpGain,
		StreakDays:    stats.StreakDays,
		AnsweredCount: answered,
		CorrectCount:  correct,
	}, nil
}

// nextStreak advances the daily streak.
//
// Same day: unchanged, so two sessions in one evening are still one day.
// Yesterday: one more. Anything older, or no record at all: the streak starts
// again at one.
func nextStreak(current int, lastActivity *string, now time.Time) int {
	if lastActivity == nil || *lastActivity == "" {
		return 1
	}
	last, err := time.Parse("2006-01-02", *lastActivity)
	if err != nil {
		return 1
	}
	today := now.UTC().Truncate(24 * time.Hour)
	switch days := int(today.Sub(last.UTC()).Hours() / 24); {
	case days <= 0:
		if current < 1 {
			return 1
		}
		return current
	case days == 1:
		return current + 1
	default:
		return 1
	}
}
