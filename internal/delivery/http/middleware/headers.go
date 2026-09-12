package middleware

import (
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// ValidateClientHeaders validates the required client headers
func ValidateClientHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only validate on /v1/ routes (skip health checks, swagger, etc.)
		platform := c.Get("X-Platform")
		if platform != "" && platform != "ios" && platform != "android" {
			return response.BadRequest(c, "INVALID_PLATFORM", "X-Platform header must be 'ios' or 'android'", nil)
		}

		locale := c.Get("Accept-Language")
		if locale != "" {
			switch locale {
			case "uz-Latn-UZ", "uz-Cyrl-UZ", "ru", "en":
				c.Locals("locale", locale)
			default:
				c.Locals("locale", "uz-Latn-UZ")
			}
		} else {
			c.Locals("locale", "uz-Latn-UZ")
		}

		return c.Next()
	}
}
