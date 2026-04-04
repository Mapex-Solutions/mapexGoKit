package customErrors

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
)

// handleValidationError handles ValidationError instances.
func handleValidationError(ctx *fiber.Ctx, err error) error {
	var validationError *ValidationError
	if !errors.As(err, &validationError) {
		return response.InternalServerError(ctx, err.Error(), err)
	}
	return response.BadRequest(ctx, validationError.Errors)
}

// handleServerCustomError handles ServerCustomError instances.
func handleServerCustomError(ctx *fiber.Ctx, err error) error {
	var serverCustomError *ServerCustomError
	if !errors.As(err, &serverCustomError) {
		return response.InternalServerError(ctx, err.Error(), err)
	}

	return response.Custom(ctx, serverCustomError.Code, serverCustomError.Errors)
}
