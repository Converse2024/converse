package middleware

import (
	"context"

	"github.com/labstack/echo/v4"
)

type RequestURLKey struct{}

func WithRequestURL(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		ctx := context.WithValue(req.Context(), RequestURLKey{}, req.URL)
		c.SetRequest(req.WithContext(ctx))
		return next(c)
	}
}
