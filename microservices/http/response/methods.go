package response

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/logger"
)

// BadRequest returns a 400 Bad Request response.
func Custom(ctx *fiber.Ctx, code int, errors []string) error {
	return ctx.Status(code).JSON(Response{
		Status: code,
		Errors: errors,
		Data:   nil,
	})
}

// BadRequest returns a 400 Bad Request response.
func BadRequest(ctx *fiber.Ctx, allErrors []string) error {

	// Create a new error for logging purposes
	err := errors.New("Bad request error")

	logger.Error(
		err,
		strings.Join(allErrors, ", "),
		logger.Field{Key: "URI", Value: ctx.OriginalURL()},
		logger.Field{Key: "Method", Value: ctx.Method()},
	)

	return ctx.Status(fiber.StatusBadRequest).JSON(Response{
		Status: fiber.StatusBadRequest,
		Errors: allErrors,
		Data:   nil,
	})
}

// Success returns a 200 OK response.
func Success(ctx *fiber.Ctx, data interface{}) error {
	return ctx.Status(fiber.StatusOK).JSON(Response{
		Status: fiber.StatusOK,
		Errors: nil,
		Data:   data,
	})
}

// Created returns a 201 Created response.
func Created(ctx *fiber.Ctx, data interface{}) error {
	return ctx.Status(fiber.StatusCreated).JSON(Response{
		Status: fiber.StatusCreated,
		Errors: nil,
		Data:   data,
	})
}

// Conflict returns a 409 Conflict response.
func Conflict(ctx *fiber.Ctx, allErrors []string) error {

	// Create a new error for logging purposes
	err := errors.New("Conflic server error")

	logger.Error(
		err,
		strings.Join(allErrors, ", "),
		logger.Field{Key: "URI", Value: ctx.OriginalURL()},
		logger.Field{Key: "Method", Value: ctx.Method()},
	)

	// Send the response
	return ctx.Status(fiber.StatusConflict).JSON(Response{
		Status: fiber.StatusConflict,
		Errors: allErrors,
		Data:   nil,
	})
}

// InternalServerError returns a 500 Internal Server Error response.
func InternalServerError(ctx *fiber.Ctx, message string, err error) error {

	if message == "" {
		message = "Internal server error occurred"
	}

	if err == nil {
		err = errors.New(message)
	}

	logger.Error(
		err,
		message,
		logger.Field{Key: "URI", Value: ctx.OriginalURL()},
		logger.Field{Key: "Method", Value: ctx.Method()},
	)

	// Send the response
	return ctx.Status(fiber.StatusInternalServerError).JSON(Response{
		Status: fiber.StatusInternalServerError,
		Errors: []string{message}, // Includes the original error message
		Data:   nil,
	})
}

// InternalServerError returns a 504 Internal Server Error response.
func TimeoutServerError(ctx *fiber.Ctx, message string, err error) error {

	if message == "" {
		message = "Internal server error occurred"
	}

	if err == nil {
		err = errors.New(message)
	}

	logger.Error(
		err,
		message,
		logger.Field{Key: "URI", Value: ctx.OriginalURL()},
		logger.Field{Key: "Method", Value: ctx.Method()},
	)

	// Send the response
	return ctx.Status(fiber.StatusGatewayTimeout).JSON(Response{
		Status: fiber.StatusGatewayTimeout,
		Errors: []string{message}, // Includes the original error message
		Data:   nil,
	})
}

// NotFound returns a 404 Not Found response.
func NotFound(ctx *fiber.Ctx, err error) error {
	errorMessage := err.Error()

	logger.Error(
		err,
		"Not found error",
		logger.Field{Key: "URI", Value: ctx.OriginalURL()},
		logger.Field{Key: "Method", Value: ctx.Method()},
	)

	return ctx.Status(fiber.StatusNotFound).JSON(Response{
		Status: fiber.StatusNotFound,
		Errors: []string{"Resource not found", errorMessage},
		Data:   nil,
	})
}
