package httpserver

import (
	"github.com/andyle182810/gframework/middleware"
	"github.com/andyle182810/gframework/transformer"
	"github.com/labstack/echo/v5"
)

const contextKeyTransformer = "transformer"

func injectTransformer(t *transformer.Transformer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Set(contextKeyTransformer, t)

			return next(c)
		}
	}
}

func transformerFromContext(c *echo.Context) *transformer.Transformer {
	if t, ok := c.Get(contextKeyTransformer).(*transformer.Transformer); ok {
		return t
	}

	return nil
}

func scrubbedBodyLogFieldExtractor(tfm *transformer.Transformer) middleware.LogFieldExtractor {
	return func(c *echo.Context) map[string]any {
		body := c.Get(middleware.ContextKeyBody)
		if body == nil {
			return nil
		}

		scrubbed, err := transformer.ScrubbedCopyValue(c.Request().Context(), tfm, body)
		if err != nil {
			return nil
		}

		return map[string]any{"body": scrubbed}
	}
}
