package customErrors

import (
	"context"
	"errors"

	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"

	"github.com/gofiber/fiber/v2"
)

// FiberErrorHandler is the global Fiber error handler.
func FiberErrorHandler(ctx *fiber.Ctx, err error) error {
	logger.Error(err, err.Error())

	fiberErr, isFiberError := err.(*fiber.Error)

	switch {
	// Timeout errors
	case isFiberError && (fiberErr.Code == fiber.StatusGatewayTimeout || fiberErr.Code == fiber.StatusRequestTimeout):
		return response.TimeoutServerError(ctx, fiberErr.Message, err)

	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
		return response.TimeoutServerError(ctx, "Request timed out", err)

	// Validation errors
	case errors.As(err, new(*ValidationError)):
		return handleValidationError(ctx, err)

	// Custom server errors
	case errors.As(err, new(*ServerCustomError)):
		return handleServerCustomError(ctx, err)

	// Not found
	case isFiberError && fiberErr.Code == fiber.StatusNotFound:
		return response.NotFound(ctx, err)

	// Default internal error
	default:
		return response.InternalServerError(ctx, "Internal error", err)
	}
}
