package health

import (
	"github.com/gofiber/fiber/v2"
	"github.com/Mapex-Solutions/mapexGoKit/microservices/http/response"
)

// Handler returns a Fiber handler that checks the health of the service.
// Returns 200 for healthy/degraded, 503 for unhealthy.
func Handler(service *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		result := service.Check(ctx)

		if result.Status == "unhealthy" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(response.Response{
				Status: fiber.StatusServiceUnavailable,
				Errors: nil,
				Data:   result,
			})
		}

		return response.Success(c, result)
	}
}
