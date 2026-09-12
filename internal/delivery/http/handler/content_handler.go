package handler

import (
	"strconv"

	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type ContentHandler struct {
	contentUsecase *usecase.ContentUsecase
}

func NewContentHandler(contentUsecase *usecase.ContentUsecase) *ContentHandler {
	return &ContentHandler{contentUsecase: contentUsecase}
}

func (h *ContentHandler) GetPacks(c *fiber.Ctx) error {
	packs, err := h.contentUsecase.GetPacks(c.Context())
	if err != nil {
		return response.Internal(c, "Failed to retrieve packs")
	}
	return c.JSON(fiber.Map{
		"items": packs,
	})
}

func (h *ContentHandler) GetPackManifest(c *fiber.Ctx) error {
	packID := c.Params("packId")
	manifest, err := h.contentUsecase.GetPackManifest(c.Context(), packID)
	if err != nil {
		return response.NotFound(c, "Pack manifest not found")
	}
	return c.JSON(manifest)
}

func (h *ContentHandler) GetQuestions(c *fiber.Ctx) error {
	packID := c.Params("packId")
	locale := c.Query("locale")
	if locale == "" {
		if l, ok := c.Locals("locale").(string); ok && l != "" {
			locale = l
		}
	}
	category := c.Query("category")
	cursor := c.Query("cursor")
	limitStr := c.Query("limit")
	limit := 20
	if limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}

	questions, nextCursor, err := h.contentUsecase.GetQuestions(c.Context(), packID, locale, category, cursor, limit)
	if err != nil {
		return response.Internal(c, "Failed to retrieve questions")
	}

	var cursorPtr *string
	if nextCursor != "" {
		cursorPtr = &nextCursor
	}

	return c.JSON(fiber.Map{
		"items":      questions,
		"nextCursor": cursorPtr,
	})
}

func (h *ContentHandler) PostOfflineDownload(c *fiber.Ctx) error {
	packID := c.Params("packId")
	userID := c.Locals("userId").(int64)

	res, err := h.contentUsecase.GetOfflineDownloadURL(c.Context(), userID, packID)
	if err != nil {
		return response.NotFound(c, "Offline pack not available")
	}

	return c.JSON(res)
}
