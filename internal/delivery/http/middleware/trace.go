package middleware

import (
	"time"

	"github.com/avtofast/avtofast-back/pkg/id"
	"github.com/avtofast/avtofast-back/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

func TraceAndLog(log *logger.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		reqId := c.Get("X-Request-Id")
		if reqId == "" {
			reqId = id.FastRequestId()
		}
		c.Locals("requestId", reqId)
		c.Set("X-Request-Id", reqId)

		err := c.Next()

		latencyMs := float64(time.Since(start).Nanoseconds()) / 1e6
		status := c.Response().StatusCode()
		userId, _ := c.Locals("userIdStr").(string)

		log.Log(logger.LogEntry{
			Level:     logger.LevelInfo,
			Message:   "http request",
			RequestId: reqId,
			Endpoint:  c.Path(),
			Method:    c.Method(),
			Status:    status,
			LatencyMs: latencyMs,
			UserId:    userId,
		})

		return err
	}
}
