package handler

import (
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// RoadSignHandler serves the road sign reference. Practising one sign's
// questions goes through POST /practice-sessions with signCode.
type RoadSignHandler struct {
	repo domain.RoadSignRepository
}

// NewRoadSignHandler takes a nil repository when signs are not stored; the
// reference is then empty.
func NewRoadSignHandler(repo domain.RoadSignRepository) *RoadSignHandler {
	return &RoadSignHandler{repo: repo}
}

func (h *RoadSignHandler) GetRoadSigns(c *fiber.Ctx) error {
	locale := c.Query("locale", domain.LocaleUzLatn)
	groups := []domain.RoadSignGroup{}
	if h.repo != nil {
		loaded, err := h.repo.ListSigns(c.Context(), locale)
		if err != nil {
			return response.Internal(c, "Failed to load road signs")
		}
		if loaded != nil {
			groups = loaded
		}
	}
	return c.JSON(fiber.Map{"groups": groups})
}
