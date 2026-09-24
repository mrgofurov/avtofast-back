package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/jwt"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	userRepo  domain.UserRepository
	jwtSecret string
}

func NewAuthHandler(userRepo domain.UserRepository, jwtSecret string) *AuthHandler {
	return &AuthHandler{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

type TelegramAuthRequest struct {
	InitData    string `json:"initData"`
	TelegramID  int64  `json:"telegramId,omitempty"`
	Username    string `json:"username,omitempty"`
	FirstName   string `json:"firstName,omitempty"`
	LastName    string `json:"lastName,omitempty"`
	PhotoURL    string `json:"photoUrl,omitempty"`
}

type TelegramUserPayload struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
	PhotoURL  string `json:"photo_url,omitempty"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  AuthUserDTO `json:"user"`
}

type AuthUserDTO struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"displayName"`
	AvatarURL   *string `json:"avatarUrl"`
	Role        string  `json:"role"`
}

// PostTelegramAuth handles authentication via Telegram WebApp initData
func (h *AuthHandler) PostTelegramAuth(c *fiber.Ctx) error {
	var req TelegramAuthRequest
	if err := c.BodyParser(&req); err != nil {
		return response.BadRequest(c, "INVALID_BODY", "Invalid JSON request payload", nil)
	}

	var tgID int64
	var firstName, lastName, username, photoURL string

	// 1. If initData query string is provided, parse and validate it
	if req.InitData != "" {
		parsedVals, err := url.ParseQuery(req.InitData)
		if err == nil {
			userJSON := parsedVals.Get("user")
			if userJSON != "" {
				var tgUser TelegramUserPayload
				if jsonErr := json.Unmarshal([]byte(userJSON), &tgUser); jsonErr == nil {
					tgID = tgUser.ID
					firstName = tgUser.FirstName
					lastName = tgUser.LastName
					username = tgUser.Username
					photoURL = tgUser.PhotoURL
				}
			}

			// Validate hash with bot token if TELEGRAM_BOT_TOKEN is set
			botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
			if botToken != "" && parsedVals.Get("hash") != "" {
				if !validateTelegramInitData(parsedVals, botToken) {
					return response.Unauthorized(c, "Telegram initData hash validation failed")
				}
			}
		}
	}

	// 2. Direct fallback fields if sent in body
	if tgID == 0 && req.TelegramID != 0 {
		tgID = req.TelegramID
		firstName = req.FirstName
		lastName = req.LastName
		username = req.Username
		photoURL = req.PhotoURL
	}

	if tgID == 0 {
		return response.BadRequest(c, "MISSING_TELEGRAM_DATA", "Could not parse valid Telegram user data", nil)
	}

	displayName := strings.TrimSpace(firstName + " " + lastName)
	if displayName == "" {
		displayName = username
	}
	if displayName == "" {
		displayName = fmt.Sprintf("Telegram User %d", tgID)
	}

	providerID := fmt.Sprintf("tg_%d", tgID)
	user, err := h.userRepo.GetOrCreateByProvider(
		c.Context(),
		"telegram",
		providerID,
		"",
		"",
		displayName,
	)
	if err != nil {
		return response.Internal(c, "Failed to authenticate or create user")
	}

	if photoURL != "" && (user.AvatarURL == nil || *user.AvatarURL == "") {
		user.AvatarURL = &photoURL
		_ = h.userRepo.Update(c.Context(), user)
	}

	// Generate JWT Token valid for 30 days
	token, err := jwt.GenerateTestToken(user.PublicID, user.Role, h.jwtSecret, 30*24*time.Hour, user.Email)
	if err != nil {
		return response.Internal(c, "Failed to generate authentication token")
	}

	return c.JSON(AuthResponse{
		Token: token,
		User: AuthUserDTO{
			ID:          user.PublicID,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
			Role:        user.Role,
		},
	})
}

type GuestAuthRequest struct {
	DisplayName string `json:"displayName,omitempty"`
	Locale      string `json:"locale,omitempty"`
}

// PostGuestAuth issues a 30-day token for browser/demo users
func (h *AuthHandler) PostGuestAuth(c *fiber.Ctx) error {
	var req GuestAuthRequest
	_ = c.BodyParser(&req)

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = "Mehmon Foydalanuvchi"
	}

	// Random guest provider ID based on timestamp
	guestProviderID := fmt.Sprintf("guest_%d", time.Now().UnixNano())
	user, err := h.userRepo.GetOrCreateByProvider(
		c.Context(),
		"guest",
		guestProviderID,
		"",
		"",
		displayName,
	)
	if err != nil {
		return response.Internal(c, "Failed to create guest session")
	}

	token, err := jwt.GenerateTestToken(user.PublicID, user.Role, h.jwtSecret, 30*24*time.Hour, user.Email)
	if err != nil {
		return response.Internal(c, "Failed to generate guest token")
	}

	return c.JSON(AuthResponse{
		Token: token,
		User: AuthUserDTO{
			ID:          user.PublicID,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
			Role:        user.Role,
		},
	})
}

// validateTelegramInitData verifies the HMAC-SHA256 signature from Telegram
func validateTelegramInitData(vals url.Values, botToken string) bool {
	expectedHash := vals.Get("hash")
	if expectedHash == "" {
		return false
	}

	// Collect and sort all keys except 'hash'
	var keys []string
	for k := range vals {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var dataCheckArr []string
	for _, k := range keys {
		dataCheckArr = append(dataCheckArr, fmt.Sprintf("%s=%s", k, vals.Get(k)))
	}
	dataCheckString := strings.Join(dataCheckArr, "\n")

	// secret_key = HMAC_SHA256("WebAppData", bot_token)
	macSecret := hmac.New(sha256.New, []byte("WebAppData"))
	macSecret.Write([]byte(botToken))
	secretKey := macSecret.Sum(nil)

	// hash = HMAC_SHA256(secret_key, data_check_string)
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(dataCheckString))
	calculatedHash := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(calculatedHash), []byte(expectedHash))
}
