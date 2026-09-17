package handler

import (
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type TopicHandler struct {
	topicUsecase *usecase.TopicUsecase
}

func NewTopicHandler(topicUsecase *usecase.TopicUsecase) *TopicHandler {
	return &TopicHandler{topicUsecase: topicUsecase}
}

// GetTopics serves the topics list: every category, how many numbered tests it
// holds, and how many of them this learner has finished.
func (h *TopicHandler) GetTopics(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	packID := c.Query("packId")
	if packID == "" {
		return response.BadRequest(c, "MISSING_PACK_ID", "packId is required", nil)
	}

	res, err := h.topicUsecase.Topics(c.Context(), userID, packID)
	if err != nil {
		return response.BadRequest(c, "TOPICS_UNAVAILABLE", err.Error(), nil)
	}
	return c.JSON(res)
}

// GetTopicTests serves one topic's numbered tests, each with the learner's best
// score on it.
func (h *TopicHandler) GetTopicTests(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	packID := c.Query("packId")
	if packID == "" {
		return response.BadRequest(c, "MISSING_PACK_ID", "packId is required", nil)
	}

	res, err := h.topicUsecase.TopicTests(c.Context(), userID, packID, c.Params("category"))
	if err != nil {
		return response.BadRequest(c, "TOPIC_TESTS_UNAVAILABLE", err.Error(), nil)
	}
	return c.JSON(res)
}
