package e2e_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A mock exam is the number a learner judges their readiness by, so it has to
// be graded from what they actually selected.
func TestE2E_MockExamGradesRealAnswers(t *testing.T) {
	s := setupE2ETest(t)

	create := s.doRequest("POST", "/v1/mock-exams", map[string]any{
		"packId": "uz-theory-2026-09",
		"locale": "uz-Latn-UZ",
		"format": "official_20",
	}, s.UserToken)
	require.Equal(t, fiber.StatusCreated, create.StatusCode)

	var exam struct {
		ID        string `json:"id"`
		Questions []struct {
			ID      string `json:"id"`
			Choices []struct {
				ID string `json:"id"`
			} `json:"choices"`
		} `json:"questions"`
	}
	require.NoError(t, json.NewDecoder(create.Body).Decode(&exam))
	require.NotEmpty(t, exam.Questions)

	// Answer every question with its first choice. The seeded bank has one
	// question whose answer is "a" and one whose answer is "b", so a correct
	// grader scores exactly one of the two.
	for _, question := range exam.Questions {
		require.NotEmpty(t, question.Choices)
		resp := s.doRequest(
			"PUT",
			"/v1/mock-exams/"+exam.ID+"/answers/"+question.ID,
			map[string]any{"selectedChoiceId": question.Choices[0].ID, "elapsedMs": 4200},
			s.UserToken,
		)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)
	}

	submit := s.doRequest("POST", "/v1/mock-exams/"+exam.ID+"/submit", nil, s.UserToken)
	require.Equal(t, fiber.StatusOK, submit.StatusCode)

	var result struct {
		CorrectCount  int  `json:"correctCount"`
		QuestionCount int  `json:"questionCount"`
		Passed        bool `json:"passed"`
		Review        []struct {
			QuestionID       string `json:"questionId"`
			SelectedChoiceID string `json:"selectedChoiceId"`
			CorrectChoiceID  string `json:"correctChoiceId"`
			IsCorrect        bool   `json:"isCorrect"`
		} `json:"review"`
	}
	require.NoError(t, json.NewDecoder(submit.Body).Decode(&result))

	assert.Less(t, result.CorrectCount, result.QuestionCount,
		"answering everything 'a' must not score full marks")
	assert.False(t, result.Passed)
	require.Len(t, result.Review, result.QuestionCount)

	for _, item := range result.Review {
		assert.Equal(t, item.SelectedChoiceID == item.CorrectChoiceID, item.IsCorrect,
			"the review's verdict must match the answer it reports")
	}
}

// An exam nobody answered scores zero, not full marks.
func TestE2E_UnansweredMockExamScoresZero(t *testing.T) {
	s := setupE2ETest(t)

	create := s.doRequest("POST", "/v1/mock-exams", map[string]any{
		"packId": "uz-theory-2026-09",
		"locale": "uz-Latn-UZ",
	}, s.UserToken)
	require.Equal(t, fiber.StatusCreated, create.StatusCode)

	var exam struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(create.Body).Decode(&exam))

	submit := s.doRequest("POST", "/v1/mock-exams/"+exam.ID+"/submit", nil, s.UserToken)
	require.Equal(t, fiber.StatusOK, submit.StatusCode)

	var result struct {
		CorrectCount int  `json:"correctCount"`
		ScorePercent int  `json:"scorePercent"`
		Passed       bool `json:"passed"`
	}
	require.NoError(t, json.NewDecoder(submit.Body).Decode(&result))

	assert.Zero(t, result.CorrectCount)
	assert.Zero(t, result.ScorePercent)
	assert.False(t, result.Passed)
}

// Practice completion records what was answered, not what was offered.
func TestE2E_PracticeCompletionCountsRealAnswers(t *testing.T) {
	s := setupE2ETest(t)

	create := s.doRequest("POST", "/v1/practice-sessions", map[string]any{
		"mode":          "adaptive",
		"packId":        "uz-theory-2026-09",
		"locale":        "uz-Latn-UZ",
		"questionCount": 10,
	}, s.UserToken)
	require.Equal(t, fiber.StatusCreated, create.StatusCode)

	var session struct {
		ID        string `json:"id"`
		Questions []struct {
			ID      string `json:"id"`
			Choices []struct {
				ID string `json:"id"`
			} `json:"choices"`
		} `json:"questions"`
	}
	require.NoError(t, json.NewDecoder(create.Body).Decode(&session))
	require.NotEmpty(t, session.Questions)

	// Answer exactly one question, then walk away — the common case for a
	// learner interrupted mid-session.
	question := session.Questions[0]
	answer := s.doRequest(
		"POST",
		"/v1/practice-sessions/"+session.ID+"/answers",
		map[string]any{
			"questionId":       question.ID,
			"selectedChoiceId": question.Choices[0].ID,
			"elapsedMs":        3000,
		},
		s.UserToken,
	)
	require.Equal(t, fiber.StatusOK, answer.StatusCode)

	complete := s.doRequest("POST", "/v1/practice-sessions/"+session.ID+"/complete", nil, s.UserToken)
	require.Equal(t, fiber.StatusOK, complete.StatusCode)

	var result struct {
		AnsweredCount int `json:"answeredCount"`
		CorrectCount  int `json:"correctCount"`
		StreakDays    int `json:"streakDays"`
	}
	require.NoError(t, json.NewDecoder(complete.Body).Decode(&result))

	assert.Equal(t, 1, result.AnsweredCount, "only one question was answered")
	assert.LessOrEqual(t, result.CorrectCount, result.AnsweredCount)
	assert.Equal(t, 1, result.StreakDays, "a first day of practice is a one-day streak")
}

// The daily goal ring tracks today, not a running total.
func TestE2E_DashboardDailyGoalTracksToday(t *testing.T) {
	s := setupE2ETest(t)

	before := s.doRequest("GET", "/v1/dashboard", nil, s.UserToken)
	require.Equal(t, fiber.StatusOK, before.StatusCode)

	var snapshot struct {
		DailyGoal struct {
			Completed int `json:"completed"`
			Target    int `json:"target"`
		} `json:"dailyGoal"`
		ReadinessScore int `json:"readinessScore"`
	}
	require.NoError(t, json.NewDecoder(before.Body).Decode(&snapshot))

	assert.Zero(t, snapshot.DailyGoal.Completed, "nothing answered today")
	assert.Positive(t, snapshot.DailyGoal.Target)
	assert.Zero(t, snapshot.ReadinessScore, "a learner who has answered nothing is not ready")
}

// Question media has to be resolvable by a client that only knows the API
// origin.
func TestE2E_QuestionMediaPathsAreResolvable(t *testing.T) {
	s := setupE2ETest(t)

	ctx := context.Background()
	require.NoError(t, s.Store.CreateQuestion(ctx, &domain.Question{
		PublicID:        "media-001",
		PackID:          "uz-theory-2026-09",
		ContentVersion:  "2026.09.1",
		Category:        domain.CategoryRoadSigns,
		Difficulty:      domain.DifficultyEasy,
		CorrectChoiceID: "a",
		Status:          "published",
		Source:          &domain.QuestionSource{Reference: "YHQ 1.1"},
		// Stored the way the scraper writes it: a bare relative path.
		Image:    &domain.QuestionImage{URL: "questions/abc.webp", SHA256: "x"},
		VideoURL: "videos/abc.mp4",
		Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {
				Prompt:  "Media savoli?",
				Choices: []domain.ChoiceItem{{ID: "a", Text: "A", Position: 1}},
			},
		},
	}))

	resp := s.doRequest("GET", "/v1/content/packs/uz-theory-2026-09/questions?limit=100", nil, s.UserToken)
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	var body struct {
		Items []struct {
			ID    string `json:"id"`
			Image *struct {
				URL string `json:"url"`
			} `json:"image"`
			VideoURL string `json:"videoUrl"`
		} `json:"items"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	var found bool
	for _, item := range body.Items {
		if item.ID != "media-001" {
			continue
		}
		found = true
		require.NotNil(t, item.Image)
		assert.Equal(t, "/medias/questions/abc.webp", item.Image.URL,
			"a client joining this to the API origin must reach the file")
		assert.Equal(t, "/medias/videos/abc.mp4", item.VideoURL)
	}
	assert.True(t, found, "the seeded media question should be listed")
}
