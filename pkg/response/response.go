package response

import (
	"github.com/gofiber/fiber/v2"
)

// ErrorDetail defines the RFC-compliant error structure
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestId string `json:"requestId"`
	Details   any    `json:"details,omitempty"`
}

// ErrorResponse defines the standard error envelope
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// SendError sends a standardized error response
func SendError(c *fiber.Ctx, statusCode int, code, message string, details any) error {
	reqId, _ := c.Locals("requestId").(string)
	if reqId == "" {
		reqId = c.Get("X-Request-Id")
	}

	return c.Status(statusCode).JSON(ErrorResponse{
		Error: ErrorDetail{
			Code:      code,
			Message:   message,
			RequestId: reqId,
			Details:   details,
		},
	})
}

// Predefined error helpers
func BadRequest(c *fiber.Ctx, code, message string, details any) error {
	if code == "" {
		code = "BAD_REQUEST"
	}
	return SendError(c, fiber.StatusBadRequest, code, message, details)
}

func Unauthorized(c *fiber.Ctx, message string) error {
	return SendError(c, fiber.StatusUnauthorized, "UNAUTHORIZED", message, nil)
}

func Forbidden(c *fiber.Ctx, code, message string) error {
	if code == "" {
		code = "FORBIDDEN"
	}
	return SendError(c, fiber.StatusForbidden, code, message, nil)
}

func NotFound(c *fiber.Ctx, message string) error {
	return SendError(c, fiber.StatusNotFound, "NOT_FOUND", message, nil)
}

func Conflict(c *fiber.Ctx, code, message string, details any) error {
	if code == "" {
		code = "CONFLICT"
	}
	return SendError(c, fiber.StatusConflict, code, message, details)
}

func Unprocessable(c *fiber.Ctx, code, message string, details any) error {
	if code == "" {
		code = "UNPROCESSABLE_ENTITY"
	}
	return SendError(c, fiber.StatusUnprocessableEntity, code, message, details)
}

func TooManyRequests(c *fiber.Ctx, retryAfterSec string) error {
	if retryAfterSec != "" {
		c.Set("Retry-After", retryAfterSec)
	}
	return SendError(c, fiber.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", "Too many requests. Please try again later.", nil)
}

func Internal(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "An unexpected error occurred"
	}
	return SendError(c, fiber.StatusInternalServerError, "INTERNAL_ERROR", message, nil)
}
