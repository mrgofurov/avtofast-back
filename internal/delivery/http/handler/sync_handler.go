package handler

import (
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type SyncHandler struct {
	syncUsecase *usecase.SyncUsecase
}

func NewSyncHandler(syncUsecase *usecase.SyncUsecase) *SyncHandler {
	return &SyncHandler{syncUsecase: syncUsecase}
}

func (h *SyncHandler) PostEvents(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req usecase.SyncUploadRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}

	res, err := h.syncUsecase.ProcessEvents(c.Context(), userID, req)
	if err != nil {
		return response.Internal(c, "Failed to process offline sync events")
	}

	return c.JSON(res)
}

func (h *SyncHandler) GetChanges(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	cursor := c.Query("cursor")

	res, err := h.syncUsecase.GetChanges(c.Context(), userID, cursor)
	if err != nil {
		return response.Internal(c, "Failed to retrieve sync changes")
	}

	return c.JSON(res)
}

type BillingHandler struct {
	billingUsecase *usecase.BillingUsecase
}

func NewBillingHandler(billingUsecase *usecase.BillingUsecase) *BillingHandler {
	return &BillingHandler{billingUsecase: billingUsecase}
}

func (h *BillingHandler) GetEntitlements(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	ent, err := h.billingUsecase.GetEntitlement(c.Context(), userID)
	if err != nil {
		return response.Internal(c, "Failed to load entitlements")
	}
	return c.JSON(ent)
}

func (h *BillingHandler) PostVerify(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	var req usecase.BillingVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid purchase verification payload", nil)
	}

	ent, err := h.billingUsecase.VerifyPurchase(c.Context(), userID, req)
	if err != nil {
		return response.BadRequest(c, "VERIFICATION_FAILED", err.Error(), nil)
	}

	return c.JSON(ent)
}

func (h *BillingHandler) PostRestore(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	ent, err := h.billingUsecase.RestorePurchases(c.Context(), userID)
	if err != nil {
		return response.Internal(c, "Failed to restore purchases")
	}
	return c.JSON(ent)
}

func (h *BillingHandler) PutDevicePushToken(c *fiber.Ctx) error {
	userID := c.Locals("userId").(int64)
	deviceID := c.Params("deviceId")
	if deviceID == "" {
		return response.BadRequest(c, "MISSING_DEVICE_ID", "deviceId path parameter is required", nil)
	}

	var req usecase.DevicePushTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid device token payload", nil)
	}

	err := h.billingUsecase.RegisterPushToken(c.Context(), userID, deviceID, req)
	if err != nil {
		return response.Internal(c, "Failed to register push token")
	}

	return c.SendStatus(fiber.StatusOK)
}
