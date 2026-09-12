package handler

import (
	"strconv"

	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type ReviewHandler struct {
	dashUsecase *usecase.DashboardUsecase
}

func NewReviewHandler(dashUsecase *usecase.DashboardUsecase) *ReviewHandler {
	return &ReviewHandler{dashUsecase: dashUsecase}
}

func (h *ReviewHandler) GetMistakes(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	dueOnly := c.Query("dueOnly") == "true"
	category := c.Query("category")
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")
	limit := 20
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	items, nextCursor, err := h.dashUsecase.GetMistakes(c.Context(), userID, dueOnly, category, cursor, limit)
	if err != nil {
		return response.Internal(c, "Failed to load mistakes")
	}

	var nextCursorPtr *string
	if nextCursor != "" {
		nextCursorPtr = &nextCursor
	}

	return c.JSON(fiber.Map{
		"items":      items,
		"nextCursor": nextCursorPtr,
	})
}
