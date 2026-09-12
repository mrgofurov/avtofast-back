package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
)

type BillingUsecase struct {
	entRepo    domain.EntitlementRepository
	deviceRepo domain.DeviceRepository
}

func NewBillingUsecase(entRepo domain.EntitlementRepository, deviceRepo domain.DeviceRepository) *BillingUsecase {
	return &BillingUsecase{entRepo: entRepo, deviceRepo: deviceRepo}
}

type EntitlementResponse struct {
	Tier      string          `json:"tier"`
	Status    string          `json:"status"`
	ProductID *string         `json:"productId"`
	ExpiresAt *string         `json:"expiresAt"`
	Features  map[string]bool `json:"features"`
}

func (u *BillingUsecase) GetEntitlement(ctx context.Context, userID int64) (*EntitlementResponse, error) {
	ent, err := u.entRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	res := &EntitlementResponse{
		Tier:      ent.Tier,
		Status:    ent.Status,
		ProductID: ent.ProductID,
		Features:  ent.Features,
	}
	if ent.ExpiresAt != nil {
		str := ent.ExpiresAt.Format(time.RFC3339)
		res.ExpiresAt = &str
	}
	return res, nil
}

type BillingVerifyRequest struct {
	Platform      string `json:"platform"` // ios | android
	ReceiptData   string `json:"receiptData"`
	TransactionID string `json:"transactionId"`
	ProductID     string `json:"productId"`
}

func (u *BillingUsecase) VerifyPurchase(ctx context.Context, userID int64, req BillingVerifyRequest) (*EntitlementResponse, error) {
	if req.Platform == "" || req.ProductID == "" {
		return nil, errors.New("platform and productId are required")
	}
	if req.TransactionID == "" {
		req.TransactionID = "tx_" + time.Now().Format("20060102150405")
	}

	// Server-side validation simulation
	_ = u.entRepo.RecordPurchase(ctx, userID, req.Platform, req.TransactionID, req.ProductID, req.ReceiptData)

	exp := time.Now().UTC().Add(30 * 24 * time.Hour)
	ent := &domain.Entitlement{
		UserID:    userID,
		Tier:      domain.TierPremium,
		Status:    "active",
		ProductID: &req.ProductID,
		ExpiresAt: &exp,
		Features: map[string]bool{
			"aiMistakeAnalysis": true,
			"advancedAnalytics": true,
			"voiceExplanations": true,
			"premiumMockExams":  true,
			"ads":               false,
		},
	}

	if err := u.entRepo.SaveEntitlement(ctx, ent); err != nil {
		return nil, err
	}

	return u.GetEntitlement(ctx, userID)
}

func (u *BillingUsecase) RestorePurchases(ctx context.Context, userID int64) (*EntitlementResponse, error) {
	return u.GetEntitlement(ctx, userID)
}

type DevicePushTokenRequest struct {
	Platform   string  `json:"platform"` // ios | android
	PushToken  *string `json:"pushToken"`
	AppVersion string  `json:"appVersion"`
	Locale     string  `json:"locale"`
	Timezone   string  `json:"timezone"`
}

func (u *BillingUsecase) RegisterPushToken(ctx context.Context, userID int64, deviceID string, req DevicePushTokenRequest) error {
	if req.PushToken == nil {
		return u.deviceRepo.DeleteDeviceToken(ctx, deviceID)
	}

	device := &domain.Device{
		PublicID:   deviceID,
		UserID:     userID,
		Platform:   req.Platform,
		PushToken:  req.PushToken,
		AppVersion: req.AppVersion,
		Locale:     req.Locale,
		Timezone:   req.Timezone,
	}
	return u.deviceRepo.UpsertDevice(ctx, device)
}
