package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/memory"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestEnvironment(t *testing.T) (*memory.MemoryStore, *config.Config) {
	cfg, err := config.Load("")
	require.NoError(t, err)

	mem := memory.New()
	ctx := context.Background()

	// Seed pack
	now := time.Now().UTC()
	pack := &domain.QuestionPack{
		PackID:          "uz-theory-2026-09",
		Version:         "2026.09.1",
		Title:           "O'zbekiston haydovchilik nazariyasi",
		Locales:         []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn},
		QuestionCount:   2,
		DownloadBytes:   18432000,
		MandatoryUpdate: false,
		Status:          "published",
		PublishedAt:     &now,
	}
	require.NoError(t, mem.CreatePack(ctx, pack))

	// Seed questions
	q1 := &domain.Question{
		PublicID:        "q-001",
		PackID:          pack.PackID,
		ContentVersion:  pack.Version,
		Category:        domain.CategoryRoadSigns,
		Difficulty:      domain.DifficultyEasy,
		CorrectChoiceID: "a",
		Status:          "published",
		Source:          &domain.QuestionSource{Reference: "YHQ 1.1"},
		Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {
				Prompt: "Savol 1?",
				Choices: []domain.ChoiceItem{
					{ID: "a", Text: "Javob A", Position: 1},
					{ID: "b", Text: "Javob B", Position: 2},
				},
				Explanation: "Chunki A to'g'ri",
			},
		},
	}
	require.NoError(t, mem.CreateQuestion(ctx, q1))

	q2 := &domain.Question{
		PublicID:        "q-002",
		PackID:          pack.PackID,
		ContentVersion:  pack.Version,
		Category:        domain.CategoryIntersections,
		Difficulty:      domain.DifficultyMedium,
		CorrectChoiceID: "b",
		Status:          "published",
		Source:          &domain.QuestionSource{Reference: "YHQ 2.1"},
		Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {
				Prompt: "Savol 2?",
				Choices: []domain.ChoiceItem{
					{ID: "a", Text: "Javob A", Position: 1},
					{ID: "b", Text: "Javob B", Position: 2},
				},
				Explanation: "Chunki B to'g'ri",
			},
		},
	}
	require.NoError(t, mem.CreateQuestion(ctx, q2))

	return mem, cfg
}

func TestPracticeSessionFlow(t *testing.T) {
	mem, _ := setupTestEnvironment(t)
	ctx := context.Background()

	user, err := mem.GetOrCreateByProvider(ctx, "firebase", "test_sub", "test@test.uz", "", "Test User")
	require.NoError(t, err)

	practiceUsecase := usecase.NewPracticeUsecase(mem, mem, mem, mem, mem)

	// 1. Create Practice Session
	session, err := practiceUsecase.CreateSession(ctx, user.ID, usecase.CreatePracticeSessionRequest{
		Mode:          domain.PracticeModeAdaptive,
		PackID:        "uz-theory-2026-09",
		Locale:        domain.LocaleUzLatn,
		QuestionCount: 2,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Len(t, session.Questions, 2)

	// 2. Submit Correct Answer
	ansRes1, err := practiceUsecase.SubmitAnswer(ctx, user.ID, session.ID, domain.PracticeAnswerSubmission{
		QuestionID:       "q-001",
		SelectedChoiceID: "a",
		ElapsedMs:        4500,
	})
	require.NoError(t, err)
	assert.True(t, ansRes1.IsCorrect)
	assert.Equal(t, "a", ansRes1.CorrectChoiceID)
	assert.Equal(t, 12, ansRes1.XpAwarded)

	// 3. Submit Incorrect Answer
	ansRes2, err := practiceUsecase.SubmitAnswer(ctx, user.ID, session.ID, domain.PracticeAnswerSubmission{
		QuestionID:       "q-002",
		SelectedChoiceID: "a", // correct is b
		ElapsedMs:        5200,
	})
	require.NoError(t, err)
	assert.False(t, ansRes2.IsCorrect)
	assert.Equal(t, "b", ansRes2.CorrectChoiceID)
	assert.NotNil(t, ansRes2.ReviewScheduledAt)

	// 4. Verify Mistake Queue has q-002
	mistakes, _, err := mem.GetDueMistakes(ctx, user.ID, false, "", "", 10)
	require.NoError(t, err)
	assert.Len(t, mistakes, 1)
	assert.Equal(t, "q-002", mistakes[0].Question.ID)
	assert.Equal(t, 1, mistakes[0].MistakeCount)

	// 5. Complete Session
	compRes, err := practiceUsecase.CompleteSession(ctx, user.ID, session.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", compRes.Status)
	assert.Greater(t, compRes.XP, 0)
}

func TestMockExamFlow(t *testing.T) {
	mem, cfg := setupTestEnvironment(t)
	ctx := context.Background()

	user, err := mem.GetOrCreateByProvider(ctx, "firebase", "user_exam", "exam@test.uz", "", "Exam User")
	require.NoError(t, err)

	cfg.Rules.ExamQuestionCount = 2
	cfg.Rules.ExamPassCount = 1
	mockExamUsecase := usecase.NewMockExamUsecase(mem, mem, mem, mem, cfg)

	// 1. Create Exam
	exam, err := mockExamUsecase.CreateExam(ctx, user.ID, usecase.CreateMockExamRequest{
		PackID: "uz-theory-2026-09",
		Locale: domain.LocaleUzLatn,
		Format: "official_20",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, exam.ID)
	assert.Equal(t, "in_progress", exam.Status)

	// 2. Submit Answer (no correctness returned for in-progress exams)
	err = mockExamUsecase.SubmitAnswer(ctx, user.ID, exam.ID, "q-001", domain.MockExamQuestionAnswer{
		SelectedChoiceID: "a",
		ElapsedMs:        3200,
	})
	require.NoError(t, err)

	// 3. Submit Exam (Server grades score)
	subRes, err := mockExamUsecase.SubmitExam(ctx, user.ID, exam.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", subRes.Status)
	assert.True(t, subRes.Passed)
	assert.Len(t, subRes.Review, 2)

	// 4. Second submission should be rejected
	_, err = mockExamUsecase.SubmitExam(ctx, user.ID, exam.ID)
	assert.Error(t, err)
}

func TestAdminValidationAndPublishing(t *testing.T) {
	mem, cfg := setupTestEnvironment(t)
	ctx := context.Background()

	admin, err := mem.GetOrCreateByProvider(ctx, "firebase", "admin_sub", "admin@avtofast.uz", "", "Admin")
	require.NoError(t, err)
	admin.Role = domain.RoleContentPublisher
	_ = mem.Update(ctx, admin)

	adminUsecase := usecase.NewAdminUsecase(mem, mem, cfg)

	// 1. Create draft question missing required Russian translation
	draftQ := &domain.Question{
		PublicID:        "draft-q-01",
		CorrectChoiceID: "a",
		Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {
				Prompt: "Prompt",
				Choices: []domain.ChoiceItem{
					{ID: "a", Text: "A", Position: 1},
					{ID: "b", Text: "B", Position: 2},
				},
				Explanation: "Expl",
			},
		},
	}
	err = adminUsecase.AddQuestion(ctx, admin.ID, "uz-theory-2026-09", draftQ)
	require.NoError(t, err)

	// 2. Validate Question (should fail because Cyrillic, Russian, English missing)
	valRes, err := adminUsecase.ValidateQuestion(ctx, "draft-q-01")
	require.NoError(t, err)
	assert.False(t, valRes.Valid)
	assert.NotEmpty(t, valRes.Errors)

	// 3. Publish Pack
	pubPack, err := adminUsecase.PublishPack(ctx, admin.ID, "uz-theory-2026-09", "Release v2026.09.1")
	require.NoError(t, err)
	assert.Equal(t, "published", pubPack.Status)
	assert.NotEmpty(t, pubPack.ManifestSHA256)
	assert.NotEmpty(t, pubPack.ManifestSignature)

	// 4. Check audit log
	logs, _, err := adminUsecase.GetAuditLogs(ctx, "", 10)
	require.NoError(t, err)
	assert.NotEmpty(t, logs)
	assert.Equal(t, "publish_pack", logs[0].Action)
}
