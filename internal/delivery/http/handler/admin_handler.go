package handler

import (
	"strconv"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type AdminHandler struct {
	adminUsecase *usecase.AdminUsecase
}

func NewAdminHandler(adminUsecase *usecase.AdminUsecase) *AdminHandler {
	return &AdminHandler{adminUsecase: adminUsecase}
}

func (h *AdminHandler) PostPackDraft(c *fiber.Ctx) error {
	actorID := c.Locals("userId").(int64)
	var req usecase.CreatePackDraftRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	pack, err := h.adminUsecase.CreateDraftPack(c.Context(), actorID, req)
	if err != nil {
		return response.BadRequest(c, "PACK_CREATION_FAILED", err.Error(), nil)
	}

	return c.Status(fiber.StatusCreated).JSON(pack)
}

func (h *AdminHandler) PostDraftQuestion(c *fiber.Ctx) error {
	actorID := c.Locals("userId").(int64)
	packID := c.Params("packId")

	var q domain.Question
	if err := c.BodyParser(&q); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	err := h.adminUsecase.AddQuestion(c.Context(), actorID, packID, &q)
	if err != nil {
		return response.BadRequest(c, "QUESTION_CREATION_FAILED", err.Error(), nil)
	}

	return c.Status(fiber.StatusCreated).JSON(q)
}

func (h *AdminHandler) PatchQuestion(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
}

func (h *AdminHandler) ValidateQuestion(c *fiber.Ctx) error {
	questionID := c.Params("questionId")
	res, err := h.adminUsecase.ValidateQuestion(c.Context(), questionID)
	if err != nil {
		return response.NotFound(c, err.Error())
	}
	return c.JSON(res)
}

type PublishPackRequest struct {
	Reason string `json:"reason"`
}

func (h *AdminHandler) PublishPack(c *fiber.Ctx) error {
	actorID := c.Locals("userId").(int64)
	packID := c.Params("packId")

	var req PublishPackRequest
	_ = c.BodyParser(&req)
	if req.Reason == "" {
		req.Reason = "Standard production release"
	}

	pack, err := h.adminUsecase.PublishPack(c.Context(), actorID, packID, req.Reason)
	if err != nil {
		return response.BadRequest(c, "PUBLISH_FAILED", err.Error(), nil)
	}

	return c.JSON(pack)
}

type RollbackPackRequest struct {
	TargetVersion string `json:"targetVersion"`
	Reason        string `json:"reason"`
}

func (h *AdminHandler) RollbackPack(c *fiber.Ctx) error {
	actorID := c.Locals("userId").(int64)
	packID := c.Params("packId")

	var req RollbackPackRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	err := h.adminUsecase.RollbackPack(c.Context(), actorID, packID, req.TargetVersion, req.Reason)
	if err != nil {
		return response.BadRequest(c, "ROLLBACK_FAILED", err.Error(), nil)
	}

	return c.SendStatus(fiber.StatusOK)
}

func (h *AdminHandler) GetAuditLog(c *fiber.Ctx) error {
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")
	limit := 20
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	logs, nextCursor, err := h.adminUsecase.GetAuditLogs(c.Context(), cursor, limit)
	if err != nil {
		return response.Internal(c, "Failed to load audit logs")
	}

	var nextCursorPtr *string
	if nextCursor != "" {
		nextCursorPtr = &nextCursor
	}

	return c.JSON(fiber.Map{
		"items":      logs,
		"nextCursor": nextCursorPtr,
	})
}
