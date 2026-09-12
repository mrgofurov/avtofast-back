package middleware

import (
	"encoding/json"
	"time"

	"github.com/avtofast/avtofast-back/internal/domain"
	"github.com/gofiber/fiber/v2"
)

type cachedResponse struct {
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}

func Idempotency(store domain.IdempotencyStore) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Only mutating methods
		method := c.Method()
		if method != fiber.MethodPost && method != fiber.MethodPut && method != fiber.MethodPatch {
			return c.Next()
		}

		key := c.Get("Idempotency-Key")
		if key == "" {
			return c.Next()
		}

		ctx := c.Context()
		userIdStr, _ := c.Locals("userIdStr").(string)
		idempKey := userIdStr + ":" + key

		// Check if already processed
		cachedBytes, exists, err := store.Get(ctx, idempKey)
		if err == nil && exists && len(cachedBytes) > 0 {
			var cr cachedResponse
			if json.Unmarshal(cachedBytes, &cr) == nil {
				c.Set("Content-Type", "application/json; charset=utf-8")
				c.Set("X-Cache-Lookup", "HIT-IDEMPOTENT")
				return c.Status(cr.Status).Send(cr.Body)
			}
		}

		err = c.Next()
		if err != nil {
			return err
		}

		// Cache successful or client-side responses (status < 500)
		status := c.Response().StatusCode()
		if status < 500 {
			body := c.Response().Body()
			cr := cachedResponse{
				Status: status,
				Body:   body,
			}
			crBytes, _ := json.Marshal(cr)
			_ = store.Set(ctx, idempKey, crBytes, 24*time.Hour)
		}

		return nil
	}
}
