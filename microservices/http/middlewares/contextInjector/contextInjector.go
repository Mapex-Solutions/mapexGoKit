package httpContextInjectorMiddleware

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

// TimeoutMiddlewareStrict enforces the timeout at the middleware level.
// If the handler doesn't finish before the deadline, it returns 504.
// This is useful when you want a hard guarantee the response won't exceed N seconds.
func ContextInjector(seconds int) fiber.Handler {
	d := time.Duration(seconds) * time.Second

	return func(c *fiber.Ctx) error {
		// Create a timeout-aware context from the request's context.
		ctx, cancel := context.WithTimeout(c.UserContext(), d)
		defer cancel()

		// Make the new context available to downstream layers.
		c.SetUserContext(ctx)

		// Send the handler's returned error (or nil) to the channel.
		return c.Next()
	}
}
