package health

import "github.com/gofiber/fiber/v2"

// RegisterRoutes registers the /health endpoint on the given Fiber app.
// The endpoint is registered before global middlewares so it requires no auth.
func RegisterRoutes(app *fiber.App, cfg Config, checkers ...CheckerConfig) *Service {
	service := NewService(cfg, checkers...)
	app.Get("/health", Handler(service))
	return service
}
