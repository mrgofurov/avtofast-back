package handler

import (
	"errors"
	"strconv"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type MockExamHandler struct {
	mockExamUsecase *usecase.MockExamUsecase
}

func NewMockExamHandler(mockExamUsecase *usecase.MockExamUsecase) *MockExamHandler {
	return &MockExamHandler{mockExamUsecase: mockExamUsecase}
}

func (h *MockExamHandler) CreateExam(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req usecase.CreateMockExamRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	if req.PackID == "" {
		return response.BadRequest(c, "MISSING_PACK_ID", "packId is required", nil)
	}

	res, err := h.mockExamUsecase.CreateExam(c.Context(), userID, req)
	if err != nil {
		return response.BadRequest(c, "EXAM_CREATION_FAILED", err.Error(), nil)
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

func (h *MockExamHandler) SubmitAnswer(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	examID := c.Params("examId")
	questionID := c.Params("questionId")

	var ans domain.MockExamQuestionAnswer
	if err := c.BodyParser(&ans); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid answer payload", nil)
	}

	err := h.mockExamUsecase.SubmitAnswer(c.Context(), userID, examID, questionID, ans)
	if err != nil {
		if errors.Is(err, usecase.ErrExamExpired) {
			return response.Conflict(c, "EXAM_EXPIRED", "Exam deadline has expired", nil)
		}
		if errors.Is(err, usecase.ErrExamCompleted) {
			return response.Conflict(c, "EXAM_COMPLETED", "Exam is already completed", nil)
		}
		return response.BadRequest(c, "SUBMISSION_FAILED", err.Error(), nil)
	}

	// No feedback is returned for exam answers
	return c.SendStatus(fiber.StatusOK)
}

func (h *MockExamHandler) SubmitExam(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	examID := c.Params("examId")

	res, err := h.mockExamUsecase.SubmitExam(c.Context(), userID, examID)
	if err != nil {
		if errors.Is(err, usecase.ErrExamCompleted) {
			return response.Conflict(c, "EXAM_ALREADY_SUBMITTED", "Exam has already been submitted", nil)
		}
		return response.BadRequest(c, "SUBMISSION_FAILED", err.Error(), nil)
	}

	return c.JSON(res)
}

func (h *MockExamHandler) GetExamHistory(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")
	limit := 20
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	exams, nextCursor, err := h.mockExamUsecase.GetExamHistory(c.Context(), userID, cursor, limit)
	if err != nil {
		return response.Internal(c, "Failed to retrieve exam history")
	}

	var nextCursorPtr *string
	if nextCursor != "" {
		nextCursorPtr = &nextCursor
	}

	return c.JSON(fiber.Map{
		"items":      exams,
		"nextCursor": nextCursorPtr,
	})
}
