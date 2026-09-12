package handler

import (
	"strconv"

	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type DashboardHandler struct {
	dashUsecase *usecase.DashboardUsecase
}

func NewDashboardHandler(dashUsecase *usecase.DashboardUsecase) *DashboardHandler {
	return &DashboardHandler{dashUsecase: dashUsecase}
}

func (h *DashboardHandler) GetDashboard(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	snapshot, err := h.dashUsecase.GetDashboard(c.Context(), userID)
	if err != nil {
		return response.Internal(c, "Failed to load dashboard")
	}
	return c.JSON(snapshot)
}

func (h *DashboardHandler) GetAnalyticsProgress(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	rangeStr := c.Query("range")
	days := 30
	if rangeStr == "7d" {
		days = 7
	} else if rangeStr == "90d" {
		days = 90
	} else if rangeStr != "" {
		if val, err := strconv.Atoi(rangeStr); err == nil && val > 0 {
			days = val
		}
	}

	progress, err := h.dashUsecase.GetAnalyticsProgress(c.Context(), userID, days)
	if err != nil {
		return response.Internal(c, "Failed to load analytics progress")
	}

	return c.JSON(progress)
}
