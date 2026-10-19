package handler

import (
	"errors"

	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/logger"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type TutorHandler struct {
	cfg          *config.Config
	tutorUsecase *usecase.TutorUsecase
	log          *logger.Logger
}

func NewTutorHandler(cfg *config.Config, tutorUsecase *usecase.TutorUsecase, log *logger.Logger) *TutorHandler {
	return &TutorHandler{cfg: cfg, tutorUsecase: tutorUsecase, log: log}
}

// PostTutor answers a learner's question from the question bank's official
// explanations, citing the YHQ clauses the answer rests on.
func (h *TutorHandler) PostTutor(c *fiber.Ctx) error {
	if !h.cfg.Features.AITutor {
		return response.Forbidden(c, "FEATURE_DISABLED", "AI tutor is currently disabled")
	}

	var req usecase.TutorRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	res, err := h.tutorUsecase.Ask(c.UserContext(), req)
	switch {
	case errors.Is(err, usecase.ErrTutorBadRequest):
		return response.BadRequest(c, "MISSING_MESSAGE", "message is required", nil)
	case errors.Is(err, usecase.ErrTutorUnavailable):
		return response.SendError(c, fiber.StatusServiceUnavailable, "AI_UNAVAILABLE", "AI tutor is not configured", nil)
	case err != nil:
		h.log.Error("tutor: answer failed", err)
		return response.SendError(c, fiber.StatusBadGateway, "AI_FAILED", "AI tutor could not answer", nil)
	}
	return c.JSON(res)
}
