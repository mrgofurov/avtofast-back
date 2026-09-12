package middleware

import (
	"strings"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/internal/usecase"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

func Authenticate(authUsecase *usecase.AuthUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return response.Unauthorized(c, "Authorization header missing")
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return response.Unauthorized(c, "Invalid Authorization header format. Expected Bearer <token>")
		}

		user, err := authUsecase.Authenticate(c.Context(), parts[1])
		if err != nil {
			return response.Unauthorized(c, "Invalid or expired authorization token")
		}

		c.Locals("user", user)
		c.Locals("userId", user.ID)
		c.Locals("userIdStr", user.PublicID)
		c.Locals("userRole", user.Role)

		return c.Next()
	}
}

func OptionalAuthenticate(authUsecase *usecase.AuthUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				if user, err := authUsecase.Authenticate(c.Context(), parts[1]); err == nil {
					c.Locals("user", user)
					c.Locals("userId", user.ID)
					c.Locals("userIdStr", user.PublicID)
					c.Locals("userRole", user.Role)
				}
			}
		}
		return c.Next()
	}
}

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, _ := c.Locals("userRole").(string)
		if userRole == "" {
			return response.Forbidden(c, "ACCESS_DENIED", "Insufficient permissions")
		}

		for _, r := range roles {
			if userRole == r || userRole == domain.RoleContentPublisher { // publisher is higher tier
				return c.Next()
			}
		}

		return response.Forbidden(c, "ACCESS_DENIED", "Administrative privileges required")
	}
}
