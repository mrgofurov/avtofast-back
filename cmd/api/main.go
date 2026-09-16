package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/delivery/http/handler"
	"github.com/avtofast/avtofast-back/internal/delivery/http/router"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/repository/memory"
	"github.com/avtofast/avtofast-back/internal/repository/postgres"
	redisrepo "github.com/avtofast/avtofast-back/internal/repository/redis"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/firebase"
	"github.com/avtofast/avtofast-back/pkg/jwt"
	"github.com/avtofast/avtofast-back/pkg/logger"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	log := logger.DefaultLogger
	log.Info("Starting AvtoFast API server...")

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		log.Error("Failed to load configuration, using defaults", err)
	}

	// 1. Database Connection (Postgres with automatic dev/fallback handling)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var userRepo domain.UserRepository
	var contentRepo domain.ContentRepository
	var practiceRepo domain.PracticeRepository
	var mockExamRepo domain.MockExamRepository
	var mistakeRepo domain.MistakeRepository
	var dashRepo domain.DashboardRepository
	var syncRepo domain.SyncRepository
	var entRepo domain.EntitlementRepository
	var deviceRepo domain.DeviceRepository
	var auditRepo domain.AuditRepository

	db, err := postgres.NewPool(ctx, cfg)
	if err == nil {
		// Verify ping
		if err := db.Pool.Ping(ctx); err == nil {
			log.Info("Connected successfully to PostgreSQL database", map[string]any{
				"host":     cfg.Postgres.Host,
				"database": cfg.Postgres.Database,
			})
			userRepo = postgres.NewUserRepository(db)
			contentRepo = postgres.NewContentRepository(db)
			practiceRepo = postgres.NewPracticeRepository(db, contentRepo.(*postgres.ContentRepository))
			mockExamRepo = postgres.NewMockExamRepository(db, contentRepo.(*postgres.ContentRepository))
			mistakeRepo = postgres.NewMistakeRepository(db, contentRepo.(*postgres.ContentRepository))
			dashRepo = postgres.NewDashboardRepository(db)
			entRepo = postgres.NewEntitlementRepository(db)
			deviceRepo = postgres.NewDeviceRepository(db)
			auditRepo = postgres.NewAuditRepository(db)
			syncRepo = postgres.NewSyncRepository(
				db,
				userRepo.(*postgres.UserRepository),
				entRepo.(*postgres.EntitlementRepository),
				dashRepo.(*postgres.DashboardRepository),
				mistakeRepo.(*postgres.MistakeRepository),
			)
		}
	}

	// Fallback to high-performance concurrent in-memory store for self-contained execution/testing
	if userRepo == nil {
		log.Info("Using high-performance in-memory repository backend")
		mem := memory.New()
		userRepo = mem
		contentRepo = mem
		practiceRepo = mem
		mockExamRepo = mem
		mistakeRepo = mem
		dashRepo = mem
		syncRepo = mem
		entRepo = mem
		deviceRepo = mem
		auditRepo = mem

		// Seed active pack in memory
		now := time.Now().UTC()
		_ = contentRepo.CreatePack(context.Background(), &domain.QuestionPack{
			PackID:          "uz-theory-2026-09",
			Version:         "2026.09.1",
			Title:           "O'zbekiston haydovchilik nazariyasi",
			Locales:         []string{domain.LocaleUzLatn, domain.LocaleUzCyrl, domain.LocaleRu, domain.LocaleEn},
			QuestionCount:   20,
			DownloadBytes:   18432000,
			MandatoryUpdate: false,
			Status:          "published",
			ManifestSHA256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			PublishedAt:     &now,
		})
	}

	// 2. Redis Cache & Idempotency Store
	var idempStore domain.IdempotencyStore
	var rateStore domain.RateLimiterStore

	rClient := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})

	rCtx, rCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rCancel()

	if err := rClient.Ping(rCtx).Err(); err == nil {
		log.Info("Connected successfully to Redis server", map[string]any{"addr": cfg.Redis.Addr})
		rStore := redisrepo.New(rClient)
		idempStore = rStore
		rateStore = rStore
	} else {
		log.Info("Redis not accessible on network; starting embedded high-speed mini-redis")
		mr, mrErr := miniredis.Run()
		if mrErr == nil {
			mrClient := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
			rStore := redisrepo.New(mrClient)
			idempStore = rStore
			rateStore = rStore
		} else {
			mem := memory.New()
			idempStore = mem
			rateStore = mem
		}
	}

	// 3. JWT Verifier
	jwtVerifier := jwt.NewVerifier(jwt.VerifierConfig{
		SecretKey:        cfg.JWT.SecretKey,
		ExpectedIssuer:   cfg.JWT.Issuer,
		ExpectedAudience: cfg.JWT.Audience,
	})

	// 3b. Session issuance: Firebase ID tokens in, AvtoFast session tokens out
	firebaseVerifier := firebase.NewVerifier(cfg.Firebase.ProjectID)
	if !firebaseVerifier.Enabled() {
		log.Info("FIREBASE_PROJECT_ID is not set; sign-in exchange is disabled")
	}
	tokenIssuer := jwt.NewIssuer(cfg.JWT.SecretKey, cfg.JWT.Issuer, cfg.JWT.Audience)

	// 4. Use Cases
	authUsecase := usecase.NewAuthUsecase(userRepo, jwtVerifier)
	sessionUsecase := usecase.NewSessionUsecase(
		userRepo,
		firebaseVerifier,
		tokenIssuer,
		jwtVerifier,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	profileUsecase := usecase.NewProfileUsecase(userRepo, entRepo)
	contentUsecase := usecase.NewContentUsecase(contentRepo, entRepo, cfg)
	practiceUsecase := usecase.NewPracticeUsecase(practiceRepo, contentRepo, mistakeRepo, dashRepo, entRepo)
	mockExamUsecase := usecase.NewMockExamUsecase(mockExamRepo, contentRepo, dashRepo, mistakeRepo, cfg)
	dashUsecase := usecase.NewDashboardUsecase(dashRepo, mistakeRepo, userRepo)
	syncUsecase := usecase.NewSyncUsecase(syncRepo)
	billingUsecase := usecase.NewBillingUsecase(entRepo, deviceRepo)
	adminUsecase := usecase.NewAdminUsecase(contentRepo, auditRepo, cfg)

	// 5. HTTP Handlers
	handlers := &router.Handlers{
		Session:   handler.NewSessionHandler(sessionUsecase),
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

	// 6. Fiber Configuration tuned for 50k RPC
	app := fiber.New(fiber.Config{
		Prefork:               cfg.App.Prefork,
		ServerHeader:          "AvtoFast-Core",
		JSONEncoder:           json.Marshal,
		JSONDecoder:           json.Unmarshal,
		ReadBufferSize:        4096,
		WriteBufferSize:       4096,
		Concurrency:           256 * 1024,
		DisableStartupMessage: false,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": fiber.Map{
					"code":    "SERVER_ERROR",
					"message": err.Error(),
				},
			})
		},
	})

	// 7. Mount Routes
	router.SetupRoutes(router.RouterConfig{
		App:         app,
		Handlers:    handlers,
		AuthUsecase: authUsecase,
		IdempStore:  idempStore,
		RateStore:   rateStore,
		Logger:      log,
	})

	// 8. Server Lifecycle & Graceful Shutdown
	addr := fmt.Sprintf(":%d", cfg.App.Port)
	go func() {
		log.Info("Server listening on " + addr)
		if err := app.Listen(addr); err != nil {
			log.Info("Server shutting down: " + err.Error())
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server gracefully...")
	_ = app.ShutdownWithTimeout(5 * time.Second)
	log.Info("Server exited cleanly.")
}
