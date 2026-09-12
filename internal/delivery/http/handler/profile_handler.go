package handler

import (
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type BootstrapHandler struct {
	contentUsecase *usecase.ContentUsecase
}

func NewBootstrapHandler(contentUsecase *usecase.ContentUsecase) *BootstrapHandler {
	return &BootstrapHandler{contentUsecase: contentUsecase}
}

func (h *BootstrapHandler) GetBootstrap(c *fiber.Ctx) error {
	res, err := h.contentUsecase.GetBootstrap(c.Context())
	if err != nil {
		return response.Internal(c, "Failed to load bootstrap configuration")
	}

	// 15-minute cacheable header as specified in doc
	c.Set("Cache-Control", "public, max-age=900")
	return c.JSON(res)
}

type ProfileHandler struct {
	profileUsecase *usecase.ProfileUsecase
}

func NewProfileHandler(profileUsecase *usecase.ProfileUsecase) *ProfileHandler {
	return &ProfileHandler{profileUsecase: profileUsecase}
}

func (h *ProfileHandler) GetMe(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	res, err := h.profileUsecase.GetProfile(c.Context(), userID)
	if err != nil {
		return response.NotFound(c, "User profile not found")
	}
	return c.JSON(res)
}

type PatchMeRequest struct {
	DisplayName *string `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"`
}

func (h *ProfileHandler) PatchMe(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req PatchMeRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	err := h.profileUsecase.UpdateProfile(c.Context(), userID, req.DisplayName, req.AvatarURL)
	if err != nil {
		return response.Internal(c, "Failed to update profile")
	}

	return h.GetMe(c)
}

type PutOnboardingRequest struct {
	AcquisitionSource string `json:"acquisitionSource"`
	KnowledgeLevel    string `json:"knowledgeLevel"`
	Locale            string `json:"locale"`
	TargetExamDate    string `json:"targetExamDate"`
	DailyQuestionGoal int    `json:"dailyQuestionGoal"`
}

func (h *ProfileHandler) PutOnboarding(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req PutOnboardingRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	if req.DailyQuestionGoal < 1 || req.DailyQuestionGoal > 100 {
		return response.BadRequest(c, "INVALID_GOAL", "dailyQuestionGoal must be between 1 and 100", nil)
	}

	// Validate targetExamDate not before today
	examDate, err := time.Parse("2006-01-02", req.TargetExamDate)
	if err != nil {
		return response.BadRequest(c, "INVALID_DATE", "targetExamDate must be in YYYY-MM-DD format", nil)
	}
	today := time.Now().Truncate(24 * time.Hour)
	if examDate.Before(today) {
		return response.BadRequest(c, "INVALID_DATE", "targetExamDate must not be in the past", nil)
	}

	ob := &domain.UserOnboarding{
		AcquisitionSource: req.AcquisitionSource,
		KnowledgeLevel:    req.KnowledgeLevel,
		Locale:            req.Locale,
		TargetExamDate:    req.TargetExamDate,
		DailyQuestionGoal: req.DailyQuestionGoal,
	}

	if err := h.profileUsecase.SaveOnboarding(c.Context(), userID, ob); err != nil {
		return response.Internal(c, "Failed to save onboarding")
	}

	return c.JSON(ob)
}

type PatchPreferencesRequest struct {
	Locale                string `json:"locale"`
	Theme                 string `json:"theme"`
	ProfileVisibility     string `json:"profileVisibility"`
	LeaderboardVisibility string `json:"leaderboardVisibility"`
}

func (h *ProfileHandler) PatchPreferences(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req PatchPreferencesRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	prefs := &domain.UserPreferences{
		Locale:                req.Locale,
		Theme:                 req.Theme,
		ProfileVisibility:     req.ProfileVisibility,
		LeaderboardVisibility: req.LeaderboardVisibility,
	}

	if err := h.profileUsecase.UpdatePreferences(c.Context(), userID, prefs); err != nil {
		return response.Internal(c, "Failed to update preferences")
	}

	return c.JSON(prefs)
}

func (h *ProfileHandler) PatchNotificationPreferences(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var np domain.NotificationPreferences
	if err := c.BodyParser(&np); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	if err := h.profileUsecase.UpdateNotificationPreferences(c.Context(), userID, &np); err != nil {
		return response.Internal(c, "Failed to update notification preferences")
	}

	return c.JSON(np)
}
