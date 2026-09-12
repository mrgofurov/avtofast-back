package benchmark_test

import (
	"bytes"
	"context"
	"io"
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
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
)

func BenchmarkBootstrapEndpoint(b *testing.B) {
	cfg, _ := config.Load("")
	mem := memory.New()
	now := time.Now().UTC()
	_ = mem.CreatePack(context.Background(), &domain.QuestionPack{
		PackID:      "uz-theory-2026-09",
		Version:     "2026.09.1",
		Title:       "Test Pack",
		Status:      "published",
		PublishedAt: &now,
	})

	contentUsecase := usecase.NewContentUsecase(mem, mem, cfg)
	authUsecase := usecase.NewAuthUsecase(mem, jwt.NewVerifier(jwt.VerifierConfig{SecretKey: "secret"}))

	app := fiber.New(fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
	})

	router.SetupRoutes(router.RouterConfig{
		App:         app,
		Handlers:    &router.Handlers{Bootstrap: handler.NewBootstrapHandler(contentUsecase)},
		AuthUsecase: authUsecase,
		IdempStore:  mem,
		RateStore:   mem,
		Logger:      logger.New(io.Discard, logger.LevelError),
	})

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/v1/bootstrap", nil)
			req.Header.Set("X-Platform", "ios")
			req.Header.Set("X-App-Version", "1.0.0")
			resp, err := app.Test(req, 1000)
			if err != nil || resp.StatusCode != 200 {
				b.Fatalf("failed request: %v, status: %d", err, resp.StatusCode)
			}
		}
	})
}

func BenchmarkQuestionSerialization(b *testing.B) {
	q := domain.ClientQuestionPayload{
		ID:             "sign-014",
		PackId:         "uz-theory-2026-09",
		ContentVersion: "2026.09.1",
		Category:       "road_signs",
		Difficulty:     "easy",
		Prompt:         "Yo'l belgisi nimani bildiradi?",
		Choices: []domain.ChoiceItem{
			{ID: "a", Text: "Asosiy yo'l", Position: 1},
			{ID: "b", Text: "Yo'l bering", Position: 2},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(q)
		if err != nil {
			b.Fatal(err)
		}
		if len(data) == 0 {
			b.Fatal("empty json")
		}
	}
}

func BenchmarkGoccyJSONUnmarshal(b *testing.B) {
	raw := []byte(`{"questionId":"sign-014","selectedChoiceId":"a","elapsedMs":4500,"answeredAt":"2026-09-09T12:35:00Z"}`)
	var sub domain.PracticeAnswerSubmission

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(raw)
		if err := json.NewDecoder(reader).Decode(&sub); err != nil {
			b.Fatal(err)
		}
	}
}
