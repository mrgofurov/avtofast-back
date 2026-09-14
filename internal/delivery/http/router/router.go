package router

import (
	"time"

	"github.com/avtofast/avtofast-back/api"
	"github.com/avtofast/avtofast-back/internal/delivery/http/handler"
	"github.com/avtofast/avtofast-back/internal/delivery/http/middleware"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Handlers struct {
	Bootstrap *handler.BootstrapHandler
	Profile   *handler.ProfileHandler
	Content   *handler.ContentHandler
	Practice  *handler.PracticeHandler
	MockExam  *handler.MockExamHandler
	Review    *handler.ReviewHandler
	Dashboard *handler.DashboardHandler
	Sync      *handler.SyncHandler
	Billing   *handler.BillingHandler
	Social    *handler.SocialHandler
	Admin     *handler.AdminHandler
}

type RouterConfig struct {
	App         *fiber.App
	Handlers    *Handlers
	AuthUsecase *usecase.AuthUsecase
	IdempStore  domain.IdempotencyStore
	RateStore   domain.RateLimiterStore
	Logger      *logger.Logger
}

func SetupRoutes(cfg RouterConfig) {
	app := cfg.App
	h := cfg.Handlers

	// Global Middlewares
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-App-Version, X-Platform, X-Device-Id, Accept-Language, Idempotency-Key, X-Request-Id",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))
	app.Use(middleware.TraceAndLog(cfg.Logger))
	app.Use(middleware.ValidateClientHeaders())

	// Health Check
	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "timestamp": time.Now().UTC()})
	})

	// OpenAPI Spec & Interactive Documentation
	app.Get("/openapi.yaml", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/yaml; charset=utf-8")
		return c.Send(api.OpenAPISpec)
	})

	app.Get("/docs", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return c.SendString(api.SwaggerUIHTML)
	})

	app.Get("/swagger", func(c *fiber.Ctx) error {
		return c.Redirect("/docs", fiber.StatusMovedPermanently)
	})

	// V1 API Group
	v1 := app.Group("/v1")

	// P0 Bootstrap (optional auth)
	v1.Get("/bootstrap", middleware.OptionalAuthenticate(cfg.AuthUsecase), h.Bootstrap.GetBootstrap)

	// User Authenticated routes
	userAuth := middleware.Authenticate(cfg.AuthUsecase)
	idemp := middleware.Idempotency(cfg.IdempStore)
	rateLimiter := middleware.RateLimit(cfg.RateStore, 120, time.Minute)

	// User profile & onboarding
	me := v1.Group("/me", userAuth)
	me.Get("", h.Profile.GetMe)
	me.Patch("", idemp, h.Profile.PatchMe)
	me.Put("/onboarding", idemp, h.Profile.PutOnboarding)
	me.Patch("/preferences", idemp, h.Profile.PatchPreferences)
	me.Patch("/notification-preferences", idemp, h.Profile.PatchNotificationPreferences)

	// Content packs & questions
	content := v1.Group("/content", userAuth)
	content.Get("/packs", h.Content.GetPacks)
	content.Get("/packs/:packId/manifest", h.Content.GetPackManifest)
	content.Get("/packs/:packId/questions", h.Content.GetQuestions)
	content.Post("/packs/:packId/offline-download", rateLimiter, h.Content.PostOfflineDownload)

	// Practice Sessions
	practice := v1.Group("/practice-sessions", userAuth)
	practice.Post("", idemp, h.Practice.CreateSession)
	practice.Post("/:sessionId/answers", idemp, h.Practice.SubmitAnswer)
	practice.Post("/:sessionId/complete", idemp, h.Practice.CompleteSession)

	// Mistake Review Queue
	v1.Get("/review/mistakes", userAuth, h.Review.GetMistakes)

	// Timed Mock Exams
	mockExams := v1.Group("/mock-exams", userAuth)
	mockExams.Post("", idemp, h.MockExam.CreateExam)
	mockExams.Put("/:examId/answers/:questionId", idemp, h.MockExam.SubmitAnswer)
	mockExams.Post("/:examId/submit", idemp, h.MockExam.SubmitExam)
	mockExams.Get("", h.MockExam.GetExamHistory)

	// Dashboard & Analytics
	v1.Get("/dashboard", userAuth, h.Dashboard.GetDashboard)
	v1.Get("/analytics/progress", userAuth, h.Dashboard.GetAnalyticsProgress)

	// Offline Sync
	syncGroup := v1.Group("/sync", userAuth, rateLimiter)
	syncGroup.Post("/events", idemp, h.Sync.PostEvents)
	syncGroup.Get("/changes", h.Sync.GetChanges)

	// Billing & Entitlements
	v1.Get("/entitlements", userAuth, h.Billing.GetEntitlements)
	billing := v1.Group("/billing", userAuth, rateLimiter)
	billing.Post("/verify", idemp, h.Billing.PostVerify)
	billing.Post("/restore", idemp, h.Billing.PostRestore)

	// Devices
	v1.Put("/devices/:deviceId/push-token", userAuth, idemp, h.Billing.PutDevicePushToken)

	// P1 / P2 Feature-Flagged Social & AI
	friends := v1.Group("/friends", userAuth)
	friends.Post("/invitations", h.Social.PostFriendInvitation)
	friends.Get("", h.Social.GetFriends)
	friends.Post("/invitations/:id/accept", h.Social.AcceptFriendInvitation)
	friends.Delete("/:friendshipId", h.Social.RemoveFriend)

	v1.Get("/leaderboards/weekly", userAuth, h.Social.GetWeeklyLeaderboard)

	challenges := v1.Group("/challenges", userAuth)
	challenges.Get("/current", h.Social.GetCurrentChallenge)
	challenges.Post("/:challengeId/join", h.Social.JoinChallenge)

	v1.Post("/speech/explanations", userAuth, h.Social.PostSpeechExplanation)

	ai := v1.Group("/ai", userAuth, rateLimiter)
	ai.Post("/mistake-analyses", h.Social.PostAIMistakeAnalysis)
	ai.Get("/mistake-analyses/:analysisId", h.Social.GetAIMistakeAnalysis)

	// Admin API (Separate audience / roles)
	adminGroup := v1.Group("/admin", userAuth)

	// Content Admin endpoints
	adminContent := adminGroup.Group("", middleware.RequireRole(domain.RoleContentAdmin, domain.RoleContentPublisher))
	adminContent.Post("/content/packs", h.Admin.PostPackDraft)
	adminContent.Post("/content/packs/:packId/questions", h.Admin.PostDraftQuestion)
	adminContent.Patch("/questions/:questionId", h.Admin.PatchQuestion)
	adminContent.Post("/questions/:questionId/validate", h.Admin.ValidateQuestion)

	// Content Publisher endpoints
	adminPublish := adminGroup.Group("", middleware.RequireRole(domain.RoleContentPublisher))
	adminPublish.Post("/content/packs/:packId/publish", h.Admin.PublishPack)
	adminPublish.Post("/content/packs/:packId/rollback", h.Admin.RollbackPack)
	adminPublish.Get("/audit-log", h.Admin.GetAuditLog)
}
