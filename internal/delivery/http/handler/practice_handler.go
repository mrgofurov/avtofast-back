package handler

import (
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type PracticeHandler struct {
	practiceUsecase *usecase.PracticeUsecase
}

func NewPracticeHandler(practiceUsecase *usecase.PracticeUsecase) *PracticeHandler {
	return &PracticeHandler{practiceUsecase: practiceUsecase}
}

func (h *PracticeHandler) CreateSession(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req usecase.CreatePracticeSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	if req.PackID == "" {
		return response.BadRequest(c, "MISSING_PACK_ID", "packId is required", nil)
	}

	res, err := h.practiceUsecase.CreateSession(c.Context(), userID, req)
	if err != nil {
		return response.BadRequest(c, "SESSION_CREATION_FAILED", err.Error(), nil)
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *PracticeHandler) SubmitAnswer(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	sessionID := c.Params("sessionId")

	var sub domain.PracticeAnswerSubmission
	if err := c.BodyParser(&sub); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid answer payload", nil)
	}

	if sub.QuestionID == "" || sub.SelectedChoiceID == "" {
		return response.BadRequest(c, "MISSING_FIELDS", "questionId and selectedChoiceId are required", nil)
	}

	res, err := h.practiceUsecase.SubmitAnswer(c.Context(), userID, sessionID, sub)
	if err != nil {
		return response.BadRequest(c, "SUBMISSION_FAILED", err.Error(), nil)
	}

	return c.JSON(res)
}

func (h *PracticeHandler) CompleteSession(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	sessionID := c.Params("sessionId")

	res, err := h.practiceUsecase.CompleteSession(c.Context(), userID, sessionID)
	if err != nil {
		return response.BadRequest(c, "COMPLETION_FAILED", err.Error(), nil)
	}

	return c.JSON(res)
}
