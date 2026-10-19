package handler

import (
	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// TicketHandler serves the exam tickets list. Starting a ticket goes through
// POST /practice-sessions with ticketNumber, like any other practice.
type TicketHandler struct {
	repo domain.TicketRepository
}

// NewTicketHandler takes a nil repository when tickets are not stored; the
// list is then empty.
func NewTicketHandler(repo domain.TicketRepository) *TicketHandler {
	return &TicketHandler{repo: repo}
}

func (h *TicketHandler) GetTickets(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	packID := c.Query("packId")
	if packID == "" {
		return response.BadRequest(c, "MISSING_PACK_ID", "packId is required", nil)
	}

	items := []domain.Ticket{}
	if h.repo != nil {
		tickets, err := h.repo.ListTickets(c.Context(), userID, packID)
		if err != nil {
			return response.Internal(c, "Failed to load tickets")
		}
		if tickets != nil {
			items = tickets
		}
	}
	return c.JSON(fiber.Map{"packId": packID, "items": items})
}
