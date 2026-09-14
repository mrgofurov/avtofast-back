package handler

import (
	"github.com/avtofast/avtofast-back/internal/config"
	"github.com/avtofast/avtofast-back/pkg/id"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type SocialHandler struct {
	cfg *config.Config
}

func NewSocialHandler(cfg *config.Config) *SocialHandler {
	return &SocialHandler{cfg: cfg}
}

// CheckFeature returns an error if feature is not enabled
func (h *SocialHandler) checkFeature(c *fiber.Ctx, featureEnabled bool, featureName string) bool {
	if !featureEnabled {
		_ = response.Forbidden(c, "FEATURE_DISABLED", featureName+" is currently disabled")
		return false
	}
	return true
}

func (h *SocialHandler) PostFriendInvitation(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.Friends, "Friends") {
		return nil
	}
	return c.JSON(fiber.Map{
		"invitationCode": "FRIEND-" + id.New(""),
		"expiresIn":      86400,
	})
}

func (h *SocialHandler) GetFriends(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.Friends, "Friends") {
		return nil
	}
	return c.JSON(fiber.Map{
		"items": []any{},
	})
}

func (h *SocialHandler) AcceptFriendInvitation(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.Friends, "Friends") {
		return nil
	}
	return c.SendStatus(fiber.StatusOK)
}

func (h *SocialHandler) RemoveFriend(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.Friends, "Friends") {
		return nil
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *SocialHandler) GetWeeklyLeaderboard(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"items": []any{},
	})
}

func (h *SocialHandler) GetCurrentChallenge(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.WeeklyChallenges, "Weekly challenges") {
		return nil
	}
	return c.JSON(fiber.Map{
		"challengeId": "week-37",
		"title":       "Haftalik sinov",
	})
}

func (h *SocialHandler) JoinChallenge(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.WeeklyChallenges, "Weekly challenges") {
		return nil
	}
	return c.SendStatus(fiber.StatusOK)
}

func (h *SocialHandler) PostSpeechExplanation(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.VoiceExplanations, "Voice explanations") {
		return nil
	}
	return c.JSON(fiber.Map{
		"audioUrl": "https://api.avtofast.uz/assets/audio/mock.mp3",
	})
}

func (h *SocialHandler) PostAIMistakeAnalysis(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.AIMistakeAnalysis, "AI Mistake Analysis") {
		return nil
	}
	return c.JSON(fiber.Map{
		"analysisId": id.New(id.PrefixAnalysis),
		"status":     "processing",
	})
}

func (h *SocialHandler) GetAIMistakeAnalysis(c *fiber.Ctx) error {
	if !h.checkFeature(c, h.cfg.Features.AIMistakeAnalysis, "AI Mistake Analysis") {
		return nil
	}
	analysisID := c.Params("analysisId")
	return c.JSON(fiber.Map{
		"analysisId": analysisID,
		"status":     "completed",
		"summary":    "Xatolar asosan chorrahalar bo'yicha",
	})
}
