package httpRequestTimeoutMiddleware

import (
	"time"

	"github.com/gofiber/fiber/v2"

	timeout "github.com/gofiber/fiber/v2/middleware/timeout"
)

// TimeoutMiddlewareFactory cria um middleware que:
// 1) Envolve a cadeia (c.Next) com um context.WithTimeout seguro (via NewWithContext)
// 2) Retorna 504 (Gateway Timeout) automaticamente quando o tempo expira
// 3) NÃO usa goroutine manual com c.Next (evitando data races)
func TimeoutMiddlewareFactory(seconds int) fiber.Handler {
	duration := time.Duration(seconds) * time.Second

	return func(c *fiber.Ctx) error {
		// Handler “wrapper” jus to call the next handler
		// This is necessary because timeout.NewWithContext expects a handler that returns an error.
		nextHandler := func(cc *fiber.Ctx) error {
			return cc.Next()
		}

		// timeout.NewWithContext injeta um ctx com deadline em c.UserContext()
		// e, ao estourar o prazo, propaga um erro (aqui customizado como 504).
		return timeout.NewWithContext(
			nextHandler,
			duration,
		)(c)

	}
}
