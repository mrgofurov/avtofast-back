package handler

import (
	"errors"

	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/firebase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// SessionHandler issues the tokens every other endpoint requires.
//
// These three routes are the only unauthenticated mutating endpoints in the
// API, which is why they sit behind their own rate limiter in the router.
type SessionHandler struct {
	sessionUsecase *usecase.SessionUsecase
}

func NewSessionHandler(sessionUsecase *usecase.SessionUsecase) *SessionHandler {
	return &SessionHandler{sessionUsecase: sessionUsecase}
}

type createSessionRequest struct {
	// IDToken is the Firebase ID token from Google or Apple sign-in.
	IDToken string `json:"idToken"`
}

// PostSession exchanges a Firebase ID token for an AvtoFast session.
func (h *SessionHandler) PostSession(c *fiber.Ctx) error {
	var req createSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}
	if req.IDToken == "" {
		return response.BadRequest(c, "MISSING_ID_TOKEN", "idToken is required", nil)
	}

	res, err := h.sessionUsecase.ExchangeFirebaseToken(c.Context(), req.IDToken)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrSignInUnavailable):
			return response.SendError(c, fiber.StatusServiceUnavailable, "SIGN_IN_UNAVAILABLE",
				"Sign-in is not available on this server", nil)
		case errors.Is(err, firebase.ErrKeyUnavailable):
			// Google's key endpoint, not the learner's token: a retry in a
			// moment is likely to work, so this must not read as "bad token".
			return response.SendError(c, fiber.StatusServiceUnavailable, "SIGN_IN_UNAVAILABLE",
				"Could not verify the sign-in token right now", nil)
		case errors.Is(err, usecase.ErrInvalidIDToken):
			return response.Unauthorized(c, "The sign-in token is invalid or has expired")
		default:
			return response.Internal(c, "Failed to establish a session")
		}
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

type guestSessionRequest struct {
	DeviceID string `json:"deviceId"`
}

// PostGuestSession issues a session for a learner who declined sign-in.
//
// The device id falls back to the `X-Device-Id` header every client already
// sends, so the body can be empty.
func (h *SessionHandler) PostGuestSession(c *fiber.Ctx) error {
	var req guestSessionRequest
	_ = c.BodyParser(&req)
	if req.DeviceID == "" {
		req.DeviceID = c.Get("X-Device-Id")
	}

	res, err := h.sessionUsecase.GuestSession(c.Context(), req.DeviceID)
	if err != nil {
		if errors.Is(err, usecase.ErrMissingDeviceID) {
			return response.BadRequest(c, "MISSING_DEVICE_ID",
				"deviceId is required, in the body or the X-Device-Id header", nil)
		}
		return response.Internal(c, "Failed to establish a guest session")
	}

	return c.Status(fiber.StatusCreated).JSON(res)
}

type refreshSessionRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// PostRefresh trades a refresh token for a new access token.
func (h *SessionHandler) PostRefresh(c *fiber.Ctx) error {
	var req refreshSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON payload", nil)
	}
	if req.RefreshToken == "" {
		return response.BadRequest(c, "MISSING_REFRESH_TOKEN", "refreshToken is required", nil)
	}

	res, err := h.sessionUsecase.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidRefreshToken) {
			return response.Unauthorized(c, "The refresh token is invalid or has expired")
		}
		return response.Internal(c, "Failed to refresh the session")
	}

	return c.JSON(res)
}
