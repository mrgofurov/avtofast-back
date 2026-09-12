package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/avtofast/avtofast-back/pkg/response"
	"github.com/gofiber/fiber/v2"
)

func RateLimit(store domain.RateLimiterStore, limit int, window time.Duration) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Identify by IP or User
		identifier := c.IP()
		if userId, ok := c.Locals("userIdStr").(string); ok && userId != "" {
			identifier = userId
		}

		key := fmt.Sprintf("%s:%s", c.Path(), identifier)
		allowed, remaining, ttl, err := store.Allow(c.Context(), key, limit, window)
		if err != nil {
			// Fail open on error
			return c.Next()
		}

		c.Set("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			retryAfterSec := strconv.Itoa(int(ttl.Seconds()) + 1)
			return response.TooManyRequests(c, retryAfterSec)
		}

		return c.Next()
	}
}
