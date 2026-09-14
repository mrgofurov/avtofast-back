package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/delivery/http/handler"
	"github.com/avtofast/avtofast-back/internal/delivery/http/router"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/memory"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/jwt"
	"github.com/avtofast/avtofast-back/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestSuite struct {
	App         *fiber.App
	Store       *memory.MemoryStore
	Config      *config.Config
	UserToken   string
	AdminToken  string
	UserID      int64
	UserPubID   string
	SecretKey   string
}

func setupE2ETest(t *testing.T) *TestSuite {
	cfg, err := config.Load("")
	require.NoError(t, err)

	secret := "test-secret-key-32-chars-long-e2e"
	cfg.JWT.SecretKey = secret
	mem := memory.New()
	ctx := context.Background()

	// Seed active pack
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
		PublicID:        "sign-014",
		PackID:          pack.PackID,
		ContentVersion:  pack.Version,
		Category:        domain.CategoryRoadSigns,
		Difficulty:      domain.DifficultyEasy,
		CorrectChoiceID: "a",
		Status:          "published",
		Source:          &domain.QuestionSource{Reference: "YHQ 2.1"},
		Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {
				Prompt: "Yo'l belgisi nimani bildiradi?",
				Choices: []domain.ChoiceItem{
					{ID: "a", Text: "Asosiy yo'l", Position: 1},
					{ID: "b", Text: "Yo'l bering", Position: 2},
				},
				Explanation: "2.1 Asosiy yo'l belgisi.",
			},
		},
	}
	require.NoError(t, mem.CreateQuestion(ctx, q1))

	q2 := &domain.Question{
		PublicID:        "cross-027",
		PackID:          pack.PackID,
		ContentVersion:  pack.Version,
		Category:        domain.CategoryIntersections,
		Difficulty:      domain.DifficultyMedium,
		CorrectChoiceID: "b",
		Status:          "published",
		Source:          &domain.QuestionSource{Reference: "YHQ 15.2"},
		Translations: map[string]domain.QuestionTranslationData{
			domain.LocaleUzLatn: {
				Prompt: "Chorraha savoli?",
				Choices: []domain.ChoiceItem{
					{ID: "a", Text: "Javob A", Position: 1},
					{ID: "b", Text: "Javob B", Position: 2},
				},
				Explanation: "15.2 bo'yicha tushuntirish.",
			},
		},
	}
	require.NoError(t, mem.CreateQuestion(ctx, q2))

	// Generate Test User & Tokens
	user, err := mem.GetOrCreateByProvider(ctx, "firebase", "sub_user_e2e", "test@avtofast.uz", "", "Mohira Karimova")
	require.NoError(t, err)

	userToken, err := jwt.GenerateTestToken(user.ProviderID, domain.RoleUser, secret, 2*time.Hour)
	require.NoError(t, err)

	adminToken, err := jwt.GenerateTestToken("sub_admin_e2e", domain.RoleContentPublisher, secret, 2*time.Hour)
	require.NoError(t, err)

	// Build Use Cases
	verifier := jwt.NewVerifier(jwt.VerifierConfig{SecretKey: secret})
	authUsecase := usecase.NewAuthUsecase(mem, verifier)
	profileUsecase := usecase.NewProfileUsecase(mem, mem)
	contentUsecase := usecase.NewContentUsecase(mem, mem, cfg)
	practiceUsecase := usecase.NewPracticeUsecase(mem, mem, mem, mem, mem)
	mockExamUsecase := usecase.NewMockExamUsecase(mem, mem, mem, mem, cfg)
	dashUsecase := usecase.NewDashboardUsecase(mem, mem, mem)
	syncUsecase := usecase.NewSyncUsecase(mem)
	billingUsecase := usecase.NewBillingUsecase(mem, mem)
	adminUsecase := usecase.NewAdminUsecase(mem, mem, cfg)

	handlers := &router.Handlers{
		Bootstrap: handler.NewBootstrapHandler(contentUsecase),
		Profile:   handler.NewProfileHandler(profileUsecase),
		Content:   handler.NewContentHandler(contentUsecase),
		Practice:  handler.NewPracticeHandler(practiceUsecase),
		MockExam:  handler.NewMockExamHandler(mockExamUsecase),
		Review:    handler.NewReviewHandler(dashUsecase),
		Dashboard: handler.NewDashboardHandler(dashUsecase),
		Sync:      handler.NewSyncHandler(syncUsecase),
		Billing:   handler.NewBillingHandler(billingUsecase),
		Social:    handler.NewSocialHandler(cfg),
		Admin:     handler.NewAdminHandler(adminUsecase),
	}

	app := fiber.New()
	router.SetupRoutes(router.RouterConfig{
		App:         app,
		Handlers:    handlers,
		AuthUsecase: authUsecase,
		IdempStore:  mem,
		RateStore:   mem,
		Logger:      logger.New(io.Discard, logger.LevelDebug),
	})

	return &TestSuite{
		App:        app,
		Store:      mem,
		Config:     cfg,
		UserToken:  userToken,
		AdminToken: adminToken,
		UserID:     user.ID,
		UserPubID:  user.PublicID,
		SecretKey:  secret,
	}
}

func (s *TestSuite) doRequest(method, url string, body any, token string, headers ...map[string]string) *http.Response {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, url, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-App-Version", "1.0.0")
	req.Header.Set("X-Platform", "ios")
	req.Header.Set("X-Device-Id", "d290f1ee-6c54-4b01-90e6-d701748f0851")
	req.Header.Set("Accept-Language", "uz-Latn-UZ")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if len(headers) > 0 {
		for k, v := range headers[0] {
			req.Header.Set(k, v)
		}
	}

	resp, _ := s.App.Test(req, 5000)
	return resp
}

func TestE2E_P0_CompleteFlow(t *testing.T) {
	s := setupE2ETest(t)

	// 0. API Documentation & OpenAPI Specification
	t.Run("GET /docs & /openapi.yaml", func(t *testing.T) {
		respDocs := s.doRequest("GET", "/docs", nil, "")
		assert.Equal(t, fiber.StatusOK, respDocs.StatusCode)
		assert.Contains(t, respDocs.Header.Get("Content-Type"), "text/html")

		respSpec := s.doRequest("GET", "/openapi.yaml", nil, "")
		assert.Equal(t, fiber.StatusOK, respSpec.StatusCode)
		assert.Contains(t, respSpec.Header.Get("Content-Type"), "text/yaml")
	})

	// 1. GET /v1/bootstrap
	t.Run("GET /v1/bootstrap", func(t *testing.T) {
		resp := s.doRequest("GET", "/v1/bootstrap", nil, "")
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Cache-Control"), "public, max-age=900")

		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		assert.Equal(t, "1.0.0", body["minimumSupportedAppVersion"])
		assert.Equal(t, "uz-Latn-UZ", body["defaultLocale"])
		assert.NotNil(t, body["examRules"])
		assert.NotNil(t, body["activeQuestionPack"])
	})

	// 2. GET /v1/me unauthorized
	t.Run("GET /v1/me Unauthorized without token", func(t *testing.T) {
		resp := s.doRequest("GET", "/v1/me", nil, "")
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})

	// 3. GET /v1/me authorized
	t.Run("GET /v1/me Authorized", func(t *testing.T) {
		resp := s.doRequest("GET", "/v1/me", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		assert.NotEmpty(t, body["id"])
		assert.Equal(t, "Mohira Karimova", body["displayName"])
	})

	// 4. PUT /v1/me/onboarding
	t.Run("PUT /v1/me/onboarding Validation", func(t *testing.T) {
		// Past date should fail with 400
		respPast := s.doRequest("PUT", "/v1/me/onboarding", map[string]any{
			"acquisitionSource": "instagram",
			"knowledgeLevel":    "beginner",
			"locale":            "uz-Latn-UZ",
			"targetExamDate":    "2020-01-01",
			"dailyQuestionGoal": 10,
		}, s.UserToken)
		assert.Equal(t, fiber.StatusBadRequest, respPast.StatusCode)

		// Future date succeeds
		respOk := s.doRequest("PUT", "/v1/me/onboarding", map[string]any{
			"acquisitionSource": "instagram",
			"knowledgeLevel":    "beginner",
			"locale":            "uz-Latn-UZ",
			"targetExamDate":    "2026-10-24",
			"dailyQuestionGoal": 15,
		}, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respOk.StatusCode)
	})

	// 5. PATCH /v1/me/preferences
	t.Run("PATCH /v1/me/preferences", func(t *testing.T) {
		resp := s.doRequest("PATCH", "/v1/me/preferences", map[string]any{
			"locale":                "uz-Latn-UZ",
			"theme":                 "dark",
			"profileVisibility":     "public",
			"leaderboardVisibility": "friends",
		}, s.UserToken)
		assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	})

	// 6. Content Packs & Questions: verify correct answers are NOT exposed
	t.Run("GET /v1/content/packs and questions", func(t *testing.T) {
		respPacks := s.doRequest("GET", "/v1/content/packs", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respPacks.StatusCode)

		respQ := s.doRequest("GET", "/v1/content/packs/uz-theory-2026-09/questions?locale=uz-Latn-UZ", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respQ.StatusCode)

		var qBody map[string]any
		_ = json.NewDecoder(respQ.Body).Decode(&qBody)
		items := qBody["items"].([]any)
		assert.NotEmpty(t, items)

		// Contract assertion: Do not expose correct answers in ordinary question-list responses!
		firstQ := items[0].(map[string]any)
		assert.Nil(t, firstQ["correctChoiceId"], "correctChoiceId must not be exposed in questions endpoint!")
		assert.NotEmpty(t, firstQ["prompt"])
		assert.NotEmpty(t, firstQ["choices"])
	})

	// 7. Practice Sessions Flow
	var sessionID string
	t.Run("Practice Session Lifecycle", func(t *testing.T) {
		// Create session
		respCreate := s.doRequest("POST", "/v1/practice-sessions", map[string]any{
			"mode":          "adaptive",
			"packId":        "uz-theory-2026-09",
			"locale":        "uz-Latn-UZ",
			"questionCount": 2,
		}, s.UserToken)
		assert.Equal(t, fiber.StatusCreated, respCreate.StatusCode)

		var sessBody map[string]any
		_ = json.NewDecoder(respCreate.Body).Decode(&sessBody)
		sessionID = sessBody["id"].(string)
		assert.NotEmpty(t, sessionID)

		// Submit answer
		respAns := s.doRequest("POST", "/v1/practice-sessions/"+sessionID+"/answers", map[string]any{
			"questionId":       "sign-014",
			"selectedChoiceId": "a",
			"elapsedMs":        4500,
		}, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respAns.StatusCode)

		var ansBody map[string]any
		_ = json.NewDecoder(respAns.Body).Decode(&ansBody)
		assert.True(t, ansBody["isCorrect"].(bool))
		assert.Equal(t, "a", ansBody["correctChoiceId"])
		assert.NotEmpty(t, ansBody["explanation"])

		// Complete session
		respComp := s.doRequest("POST", "/v1/practice-sessions/"+sessionID+"/complete", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respComp.StatusCode)
	})

	// 8. Timed Mock Exam Lifecycle
	t.Run("Timed Mock Exam Lifecycle", func(t *testing.T) {
		// Create Exam
		respCreate := s.doRequest("POST", "/v1/mock-exams", map[string]any{
			"packId": "uz-theory-2026-09",
			"locale": "uz-Latn-UZ",
			"format": "official_20",
		}, s.UserToken)
		assert.Equal(t, fiber.StatusCreated, respCreate.StatusCode)

		var examBody map[string]any
		_ = json.NewDecoder(respCreate.Body).Decode(&examBody)
		examID := examBody["id"].(string)
		assert.NotEmpty(t, examID)
		assert.Equal(t, "in_progress", examBody["status"])

		// Persist Answer without feedback
		respAns := s.doRequest("PUT", "/v1/mock-exams/"+examID+"/answers/sign-014", map[string]any{
			"selectedChoiceId": "a",
			"elapsedMs":        5000,
		}, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respAns.StatusCode)

		// Submit Exam (Server grades score)
		respSubmit := s.doRequest("POST", "/v1/mock-exams/"+examID+"/submit", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respSubmit.StatusCode)

		var subBody map[string]any
		_ = json.NewDecoder(respSubmit.Body).Decode(&subBody)
		assert.Equal(t, "completed", subBody["status"])
		assert.NotNil(t, subBody["passed"])
		assert.NotNil(t, subBody["scorePercent"])
		assert.NotEmpty(t, subBody["review"])

		// Attempt history
		respHist := s.doRequest("GET", "/v1/mock-exams", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respHist.StatusCode)
	})

	// 9. Dashboard Snapshot & Analytics
	t.Run("GET /v1/dashboard & /v1/analytics/progress", func(t *testing.T) {
		respDash := s.doRequest("GET", "/v1/dashboard", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respDash.StatusCode)

		var dashBody map[string]any
		_ = json.NewDecoder(respDash.Body).Decode(&dashBody)
		assert.NotNil(t, dashBody["xp"])
		assert.NotNil(t, dashBody["streakDays"])
		assert.NotNil(t, dashBody["readinessScore"])
		assert.NotEmpty(t, dashBody["nextRecommendedAction"])

		respProg := s.doRequest("GET", "/v1/analytics/progress?range=30d", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respProg.StatusCode)
	})

	// 10. Offline Sync
	t.Run("POST /v1/sync/events and GET /v1/sync/changes", func(t *testing.T) {
		respSync := s.doRequest("POST", "/v1/sync/events", map[string]any{
			"events": []map[string]any{
				{
					"id":         "evt_01J_test",
					"type":       "practice_answered",
					"occurredAt": time.Now().UTC().Format(time.RFC3339),
					"packId":     "uz-theory-2026-09",
					"payload": map[string]any{
						"questionId": "sign-014",
						"selectedChoiceId": "a",
					},
				},
			},
		}, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respSync.StatusCode)

		respChanges := s.doRequest("GET", "/v1/sync/changes", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respChanges.StatusCode)
	})

	// 11. Entitlements & Billing
	t.Run("Billing verification and entitlements", func(t *testing.T) {
		respEnt := s.doRequest("GET", "/v1/entitlements", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respEnt.StatusCode)

		respVerify := s.doRequest("POST", "/v1/billing/verify", map[string]any{
			"platform":      "ios",
			"productId":     "autofast.monthly",
			"transactionId": "tx_apple_12345",
			"receiptData":   "base64-apple-receipt",
		}, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respVerify.StatusCode)

		var verifyBody map[string]any
		_ = json.NewDecoder(respVerify.Body).Decode(&verifyBody)
		assert.Equal(t, "premium", verifyBody["tier"])
		assert.Equal(t, "active", verifyBody["status"])

		// Restore
		respRestore := s.doRequest("POST", "/v1/billing/restore", nil, s.UserToken)
		assert.Equal(t, fiber.StatusOK, respRestore.StatusCode)
	})

	// 12. Idempotency Test: same Idempotency-Key returns cached response
	t.Run("Idempotency 24h caching", func(t *testing.T) {
		headers := map[string]string{
			"Idempotency-Key": "4c94d21e-c75c-4467-8857-c812d31221bb",
		}

		resp1 := s.doRequest("POST", "/v1/practice-sessions", map[string]any{
			"mode":          "adaptive",
			"packId":        "uz-theory-2026-09",
			"locale":        "uz-Latn-UZ",
			"questionCount": 2,
		}, s.UserToken, headers)
		assert.Equal(t, fiber.StatusCreated, resp1.StatusCode)

		body1, _ := io.ReadAll(resp1.Body)

		// Second request with identical Idempotency-Key
		resp2 := s.doRequest("POST", "/v1/practice-sessions", map[string]any{
			"mode":          "adaptive",
			"packId":        "uz-theory-2026-09",
			"locale":        "uz-Latn-UZ",
			"questionCount": 2,
		}, s.UserToken, headers)
		assert.Equal(t, fiber.StatusCreated, resp2.StatusCode)

		body2, _ := io.ReadAll(resp2.Body)
		assert.Equal(t, string(body1), string(body2), "Idempotent response must match original response!")
	})

	// 13. Admin Pack Authoring & Publishing
	t.Run("Admin Pack Authoring & Publishing Flow", func(t *testing.T) {
		// Non-admin token should be forbidden
		respForbidden := s.doRequest("POST", "/v1/admin/content/packs", map[string]any{
			"id":      "uz-theory-2026-10",
			"version": "2026.10.1",
			"title":   "October Release",
		}, s.UserToken)
		assert.Equal(t, fiber.StatusForbidden, respForbidden.StatusCode)

		// Admin token creates pack
		respAdmin := s.doRequest("POST", "/v1/admin/content/packs", map[string]any{
			"id":      "uz-theory-2026-10",
			"version": "2026.10.1",
			"title":   "October Release",
		}, s.AdminToken)
		assert.Equal(t, fiber.StatusCreated, respAdmin.StatusCode)

		// Admin publishes pack
		respPub := s.doRequest("POST", "/v1/admin/content/packs/uz-theory-2026-10/publish", map[string]any{
			"reason": "Official monthly release",
		}, s.AdminToken)
		assert.Equal(t, fiber.StatusOK, respPub.StatusCode)

		// Admin checks audit logs
		respAudit := s.doRequest("GET", "/v1/admin/audit-log", nil, s.AdminToken)
		assert.Equal(t, fiber.StatusOK, respAudit.StatusCode)
	})
}
