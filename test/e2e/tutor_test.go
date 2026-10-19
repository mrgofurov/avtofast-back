package e2e_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/avtofast/avtofast-back/internal/delivery/http/handler"
	"github.com/avtofast/avtofast-back/internal/delivery/http/router"
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/jwt"
	"github.com/avtofast/avtofast-back/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTutorModel struct{ reply domain.TutorModelAnswer }

func (s stubTutorModel) Answer(context.Context, domain.TutorModelRequest) (*domain.TutorModelAnswer, error) {
	r := s.reply
	return &r, nil
}

// withTutor mounts the tutor route on a fresh app sharing the suite's store
// and tokens.
func withTutor(t *testing.T, s *TestSuite, enabled bool, model domain.TutorModel) {
	t.Helper()
	s.Config.Features.AITutor = enabled
	app := fiber.New()
	log := logger.New(io.Discard, logger.LevelDebug)
	router.SetupRoutes(router.RouterConfig{
		App: app,
		Handlers: &router.Handlers{
			Tutor: handler.NewTutorHandler(s.Config, usecase.NewTutorUsecase(s.Store, nil, model), log),
		},
		AuthUsecase: usecase.NewAuthUsecase(s.Store, jwt.NewVerifier(jwt.VerifierConfig{SecretKey: s.SecretKey})),
		IdempStore:  s.Store,
		RateStore:   s.Store,
		Logger:      log,
	})
	s.App = app
}

func TestE2E_Tutor(t *testing.T) {
	s := setupE2ETest(t)
	body := map[string]any{"message": "Asosiy yo'l belgisi nimani bildiradi?", "questionId": "sign-014", "selectedChoiceId": "b"}

	t.Run("requires auth", func(t *testing.T) {
		withTutor(t, s, true, stubTutorModel{})
		resp := s.doRequest("POST", "/v1/ai/tutor", body, "")
		assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("feature flag off", func(t *testing.T) {
		withTutor(t, s, false, stubTutorModel{})
		resp := s.doRequest("POST", "/v1/ai/tutor", body, s.UserToken)
		assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
	})

	t.Run("no API key", func(t *testing.T) {
		withTutor(t, s, true, nil)
		resp := s.doRequest("POST", "/v1/ai/tutor", body, s.UserToken)
		assert.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)
	})

	t.Run("empty message", func(t *testing.T) {
		withTutor(t, s, true, stubTutorModel{})
		resp := s.doRequest("POST", "/v1/ai/tutor", map[string]any{"message": " "}, s.UserToken)
		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})

	t.Run("grounded answer carries the cited explanation", func(t *testing.T) {
		withTutor(t, s, true, stubTutorModel{reply: domain.TutorModelAnswer{
			Answer: "Bu 2.1 Asosiy yo'l belgisi.", Grounded: true, UsedSources: []int{1},
		}})
		resp := s.doRequest("POST", "/v1/ai/tutor", body, s.UserToken)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		var res usecase.TutorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.True(t, res.Grounded)
		require.Len(t, res.Citations, 1)
		assert.Equal(t, "sign-014", res.Citations[0].QuestionID)
		assert.Equal(t, "2.1 Asosiy yo'l belgisi.", res.Citations[0].Text)
	})

	t.Run("ungrounded answer is the fixed not-found text", func(t *testing.T) {
		withTutor(t, s, true, stubTutorModel{reply: domain.TutorModelAnswer{Answer: "Taxmin", Grounded: false}})
		resp := s.doRequest("POST", "/v1/ai/tutor", body, s.UserToken)
		require.Equal(t, fiber.StatusOK, resp.StatusCode)

		var res usecase.TutorResponse
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&res))
		assert.False(t, res.Grounded)
		assert.Equal(t, usecase.TutorNotFoundAnswer, res.Answer)
		assert.NotNil(t, res.Citations)
		assert.Empty(t, res.Citations)
	})
}
