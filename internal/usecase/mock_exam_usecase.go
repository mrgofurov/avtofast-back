package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/id"
)

var (
	ErrExamExpired   = errors.New("EXAM_EXPIRED")
	ErrExamCompleted = errors.New("EXAM_ALREADY_COMPLETED")
)

// examCompletionXP is paid once for sitting the exam through, on top of the
// per-answer XP, because finishing a timed twenty is its own achievement.
const examCompletionXP = 50

type MockExamUsecase struct {
	examRepo    domain.MockExamRepository
	contentRepo domain.ContentRepository
	dashRepo    domain.DashboardRepository
	mistakeRepo domain.MistakeRepository
	cfg         *config.Config
}

func NewMockExamUsecase(
	examRepo domain.MockExamRepository,
	contentRepo domain.ContentRepository,
	dashRepo domain.DashboardRepository,
	mistakeRepo domain.MistakeRepository,
	cfg *config.Config,
) *MockExamUsecase {
	return &MockExamUsecase{
		examRepo:    examRepo,
		contentRepo: contentRepo,
		dashRepo:    dashRepo,
		mistakeRepo: mistakeRepo,
		cfg:         cfg,
	}
}

type CreateMockExamRequest struct {
	PackID string `json:"packId"`
	Locale string `json:"locale"`
	Format string `json:"format"`
}

type CreateMockExamResponse struct {
	ID               string                         `json:"id"`
	Status           string                         `json:"status"`
	StartedAt        string                         `json:"startedAt"`
	DeadlineAt       string                         `json:"deadlineAt"`
	PassCorrectCount int                            `json:"passCorrectCount"`
	Questions        []domain.ClientQuestionPayload `json:"questions"`
}

func (u *MockExamUsecase) CreateExam(ctx context.Context, userID int64, req CreateMockExamRequest) (*CreateMockExamResponse, error) {
	if req.Locale == "" {
		req.Locale = domain.LocaleUzLatn
	}
	if req.Format == "" {
		req.Format = "official_20"
	}

	pack, err := u.contentRepo.GetPackByID(ctx, req.PackID)
	if err != nil || pack == nil {
		return nil, errors.New("question pack not found")
	}

	qCount := u.cfg.Rules.ExamQuestionCount
	if qCount <= 0 {
		qCount = 20
	}

	questions, err := u.contentRepo.GetRandomQuestions(ctx, req.PackID, qCount, "")
	if err != nil || len(questions) == 0 {
		return nil, errors.New("insufficient questions to generate mock exam")
	}

	now := time.Now().UTC()
	deadline := now.Add(time.Duration(u.cfg.Rules.ExamDurationSec) * time.Second)

	exam := &domain.MockExam{
		PublicID:         id.New(id.PrefixExam),
		UserID:           userID,
		PackID:           req.PackID,
		PackVersion:      pack.Version,
		Locale:           req.Locale,
		Format:           req.Format,
		Status:           domain.ExamStatusInProgress,
		StartedAt:        now,
		DeadlineAt:       deadline,
		QuestionCount:    len(questions),
		PassCorrectCount: u.cfg.Rules.ExamPassCount,
		Questions:        questions,
	}

	if err := u.examRepo.CreateExam(ctx, exam); err != nil {
		return nil, err
	}

	var clientQuestions []domain.ClientQuestionPayload
	for _, q := range questions {
		clientQuestions = append(clientQuestions, q.ToClientPayload(req.Locale))
	}

	return &CreateMockExamResponse{
		ID:               exam.PublicID,
		Status:           exam.Status,
		StartedAt:        exam.StartedAt.Format(time.RFC3339),
		DeadlineAt:       exam.DeadlineAt.Format(time.RFC3339),
		PassCorrectCount: exam.PassCorrectCount,
		Questions:        clientQuestions,
	}, nil
}

func (u *MockExamUsecase) SubmitAnswer(ctx context.Context, userID int64, examPublicID string, questionPublicID string, ans domain.MockExamQuestionAnswer) error {
	exam, err := u.examRepo.GetExamByPublicID(ctx, examPublicID)
	if err != nil || exam == nil {
		return errors.New("exam not found")
	}
	if exam.UserID != userID {
		return errors.New("forbidden")
	}
	if exam.Status != domain.ExamStatusInProgress {
		return ErrExamCompleted
	}
	if time.Now().UTC().After(exam.DeadlineAt) {
		return ErrExamExpired
	}

	var targetQ *domain.Question
	for _, q := range exam.Questions {
		if q.PublicID == questionPublicID {
			targetQ = q
			break
		}
	}
	if targetQ == nil {
		return errors.New("question not found in exam")
	}

	return u.examRepo.SaveExamAnswer(ctx, exam.ID, targetQ.ID, ans.SelectedChoiceID, ans.ElapsedMs)
}

func (u *MockExamUsecase) SubmitExam(ctx context.Context, userID int64, examPublicID string) (*domain.MockExamSubmissionResult, error) {
	exam, err := u.examRepo.GetExamByPublicID(ctx, examPublicID)
	if err != nil || exam == nil {
		return nil, errors.New("exam not found")
	}
	if exam.UserID != userID {
		return nil, errors.New("forbidden")
	}
	if exam.Status == domain.ExamStatusCompleted {
		return nil, ErrExamCompleted
	}

	now := time.Now().UTC()

	// Grading reads the stored answers. Anything else — assuming the learner
	// answered, or assuming they answered correctly — reports a score that is
	// not theirs, and this exam is the number they judge their readiness by.
	answers, err := u.examRepo.GetExamAnswers(ctx, exam.ID)
	if err != nil {
		return nil, err
	}

	reviews := make([]domain.MockExamQuestionReview, 0, len(exam.Questions))
	correctCount := 0

	for _, q := range exam.Questions {
		tr := q.Translations[exam.Locale]
		selectedChoice, answered := answers[q.ID]
		isCorrect := answered && selectedChoice == q.CorrectChoiceID

		if isCorrect {
			correctCount++
		} else {
			// An unanswered question is a gap in the same way a wrong one is,
			// so both go into the review queue.
			_ = u.mistakeRepo.UpsertMistake(ctx, userID, q.ID, false)
		}
		_ = u.examRepo.SetExamAnswerCorrectness(ctx, exam.ID, q.ID, isCorrect)

		reference := ""
		if q.Source != nil {
			reference = q.Source.Reference
		}

		reviews = append(reviews, domain.MockExamQuestionReview{
			QuestionID:       q.PublicID,
			SelectedChoiceID: selectedChoice,
			CorrectChoiceID:  q.CorrectChoiceID,
			IsCorrect:        isCorrect,
			Explanation:      tr.Explanation,
			Reference:        reference,
		})
	}

	scorePercent := 0
	if exam.QuestionCount > 0 {
		scorePercent = (correctCount * 100) / exam.QuestionCount
	}
	passed := correctCount >= exam.PassCorrectCount

	_ = u.examRepo.CompleteExam(ctx, exam.ID, correctCount, scorePercent, passed, now)

	// Update user stats
	stats, _ := u.dashRepo.GetUserStats(ctx, userID)
	if stats != nil {
		stats.CompletedMockExams++
		if passed {
			stats.PassedMockExams++
		}
		// An exam's answers count towards accuracy like any other: leaving
		// them out let a learner's dashboard accuracy drift away from how
		// they actually perform under exam conditions.
		stats.TotalAnswered += len(answers)
		stats.TotalCorrect += correctCount
		stats.XP += examCompletionXP + correctCount*xpPerCorrectAnswer
		stats.Level = stats.XP/200 + 1
		stats.StreakDays = nextStreak(stats.StreakDays, stats.LastActivityDate, now)
		_ = u.dashRepo.UpdateUserStats(ctx, stats)
	}

	return &domain.MockExamSubmissionResult{
		Status:           domain.ExamStatusCompleted,
		CorrectCount:     correctCount,
		QuestionCount:    exam.QuestionCount,
		PassCorrectCount: exam.PassCorrectCount,
		Passed:           passed,
		ScorePercent:     scorePercent,
		CompletedAt:      now,
		Review:           reviews,
	}, nil
}

func (u *MockExamUsecase) GetExamHistory(ctx context.Context, userID int64, cursor string, limit int) ([]*domain.MockExam, string, error) {
	return u.examRepo.GetUserExams(ctx, userID, cursor, limit)
}
