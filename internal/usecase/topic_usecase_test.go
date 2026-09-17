package usecase_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/memory"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const topicPackID = "uz-theory-topics"

// seedTopicBank builds a pack big enough to be cut into several tests, which is
// the only size at which slicing means anything.
func seedTopicBank(t *testing.T, roadSigns, firstAid int) *memory.MemoryStore {
	t.Helper()
	ctx := context.Background()
	mem := memory.New()

	now := time.Now().UTC()
	require.NoError(t, mem.CreatePack(ctx, &domain.QuestionPack{
		PackID:      topicPackID,
		Version:     "2026.09.1",
		Locales:     []string{domain.LocaleUzLatn},
		Status:      "published",
		PublishedAt: &now,
	}))

	add := func(category string, count int) {
		for i := 1; i <= count; i++ {
			require.NoError(t, mem.CreateQuestion(ctx, &domain.Question{
				PublicID:        fmt.Sprintf("%s-%04d", category, i),
				PackID:          topicPackID,
				ContentVersion:  "2026.09.1",
				Category:        category,
				Difficulty:      domain.DifficultyEasy,
				CorrectChoiceID: "a",
				Status:          "published",
				// Seeded because SubmitAnswer reads Source.Reference without a
				// nil check; the Postgres store always fills it in.
				Source: &domain.QuestionSource{Reference: fmt.Sprintf("YHQ %d", i)},
				Translations: map[string]domain.QuestionTranslationData{
					domain.LocaleUzLatn: {
						Prompt: fmt.Sprintf("%s savol %d", category, i),
						Choices: []domain.ChoiceItem{
							{ID: "a", Text: "A", Position: 1},
							{ID: "b", Text: "B", Position: 2},
						},
					},
				},
			}))
		}
	}
	add(domain.CategoryRoadSigns, roadSigns)
	add(domain.CategoryFirstAid, firstAid)
	return mem
}

func TestTopicsCutTheWholeBankIntoNumberedTests(t *testing.T) {
	// 47 road-sign questions is two full tests and a short third, which is the
	// case a floor division would silently drop.
	mem := seedTopicBank(t, 47, 20)
	topics := usecase.NewTopicUsecase(mem, mem, mem)

	res, err := topics.Topics(context.Background(), 1, topicPackID)
	require.NoError(t, err)
	assert.Equal(t, domain.QuestionsPerTest, res.QuestionsPerTest)
	// Every category of the syllabus is listed, including the five with no
	// questions in this pack.
	require.Len(t, res.Items, len(domain.Categories))

	byCategory := map[string]domain.TopicSummary{}
	for _, item := range res.Items {
		byCategory[item.Category] = item
	}
	assert.Equal(t, 47, byCategory[domain.CategoryRoadSigns].QuestionCount)
	assert.Equal(t, 3, byCategory[domain.CategoryRoadSigns].TestCount)
	assert.Equal(t, 1, byCategory[domain.CategoryFirstAid].TestCount)
	assert.Equal(t, 0, byCategory[domain.CategorySituations].TestCount)

	tests, err := topics.TopicTests(context.Background(), 1, topicPackID, domain.CategoryRoadSigns)
	require.NoError(t, err)
	require.Len(t, tests.Items, 3)
	assert.Equal(t, 20, tests.Items[0].QuestionCount)
	assert.Equal(t, 20, tests.Items[1].QuestionCount)
	// The remainder, not a padded or dropped test.
	assert.Equal(t, 7, tests.Items[2].QuestionCount)

	// Every question in the bank is reachable through exactly one test.
	var total int
	for _, test := range tests.Items {
		total += test.QuestionCount
	}
	assert.Equal(t, 47, total)
}

func TestNumberedTestsAreTheSameQuestionsEveryTime(t *testing.T) {
	mem := seedTopicBank(t, 47, 0)
	practice := usecase.NewPracticeUsecase(mem, mem, mem, mem, mem)
	ctx := context.Background()
	category := domain.CategoryRoadSigns

	ask := func(index int) []string {
		testIndex := index
		res, err := practice.CreateSession(ctx, 1, usecase.CreatePracticeSessionRequest{
			Mode:      domain.PracticeModeCategory,
			PackID:    topicPackID,
			Locale:    domain.LocaleUzLatn,
			Category:  &category,
			TestIndex: &testIndex,
		})
		require.NoError(t, err)
		ids := make([]string, 0, len(res.Questions))
		for _, q := range res.Questions {
			ids = append(ids, q.ID)
		}
		return ids
	}

	first := ask(2)
	require.Len(t, first, 20)
	assert.Equal(t, first, ask(2), "the same test handed out twice must be the same questions")

	// Neighbouring tests do not overlap, and between them they cover the bank
	// in order.
	assert.Equal(t, "road_signs-0001", ask(1)[0])
	assert.Equal(t, "road_signs-0021", first[0])
	assert.Equal(t, "road_signs-0041", ask(3)[0])
	assert.Len(t, ask(3), 7)

	// A test past the end of the category is an error, not an empty session
	// the learner would be dropped into.
	tooFar := 4
	_, err := practice.CreateSession(ctx, 1, usecase.CreatePracticeSessionRequest{
		Mode:      domain.PracticeModeCategory,
		PackID:    topicPackID,
		Locale:    domain.LocaleUzLatn,
		Category:  &category,
		TestIndex: &tooFar,
	})
	assert.Error(t, err)
}

func TestTopicTestsReportTheLearnersBestAttempt(t *testing.T) {
	mem := seedTopicBank(t, 40, 0)
	practice := usecase.NewPracticeUsecase(mem, mem, mem, mem, mem)
	topics := usecase.NewTopicUsecase(mem, mem, mem)
	ctx := context.Background()
	category := domain.CategoryRoadSigns
	const userID int64 = 7

	// Sit Test 1 twice, answering better the first time than the second.
	sit := func(correctAnswers int) {
		testIndex := 1
		session, err := practice.CreateSession(ctx, userID, usecase.CreatePracticeSessionRequest{
			Mode:      domain.PracticeModeCategory,
			PackID:    topicPackID,
			Locale:    domain.LocaleUzLatn,
			Category:  &category,
			TestIndex: &testIndex,
		})
		require.NoError(t, err)
		for i, question := range session.Questions {
			choice := "b"
			if i < correctAnswers {
				choice = "a"
			}
			_, err := practice.SubmitAnswer(ctx, userID, session.ID, domain.PracticeAnswerSubmission{
				QuestionID:       question.ID,
				SelectedChoiceID: choice,
				ElapsedMs:        1200,
			})
			require.NoError(t, err)
		}
		_, err = practice.CompleteSession(ctx, userID, session.ID)
		require.NoError(t, err)
	}
	sit(18)
	sit(11)

	res, err := topics.TopicTests(ctx, userID, topicPackID, category)
	require.NoError(t, err)
	require.Len(t, res.Items, 2)

	require.NotNil(t, res.Items[0].BestCorrect)
	assert.Equal(t, 18, *res.Items[0].BestCorrect, "a retry must not overwrite a better score")
	assert.Equal(t, 2, res.Items[0].Attempts)
	assert.NotNil(t, res.Items[0].LastAttemptAt)

	// Test 2 was never sat, and says so rather than reporting zero correct.
	assert.Nil(t, res.Items[1].BestCorrect)
	assert.Equal(t, 0, res.Items[1].Attempts)

	// Two attempts at one test is one test covered, not two.
	summary, err := topics.Topics(ctx, userID, topicPackID)
	require.NoError(t, err)
	for _, item := range summary.Items {
		if item.Category == category {
			assert.Equal(t, 1, item.CompletedTests)
		}
	}
}

func TestTopicTestsRejectAnUnknownCategory(t *testing.T) {
	mem := seedTopicBank(t, 20, 0)
	topics := usecase.NewTopicUsecase(mem, mem, mem)

	_, err := topics.TopicTests(context.Background(), 1, topicPackID, "parallel_parking")
	assert.Error(t, err)
}
