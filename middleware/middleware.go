package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type RequestURLKey struct{}

func ServerDelay(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		time.Sleep(2000 * time.Millisecond)
		return next(c)
	}
}

func WithRequestURL(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		req := c.Request()
		ctx := context.WithValue(req.Context(), RequestURLKey{}, req.URL)
		c.SetRequest(req.WithContext(ctx))
		return next(c)
	}
}

func HXRedirectMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Process the request
		if err := next(c); err != nil {
			c.Error(err)
		}

		// After the next handler has processed the request, check for HX-Request header
		if c.Request().Header.Get("HX-Request") != "" && c.Response().Header().Get(echo.HeaderLocation) != "" {
			c.Response().Header().Set("HX-Redirect", c.Response().Header().Get(echo.HeaderLocation))
			c.Response().Header().Del(echo.HeaderLocation) // Remove Location header to avoid double redirection
			return c.JSON(http.StatusSeeOther, map[string]string{
				"redirect": c.Response().Header().Get("HX-Redirect"),
			})
		}

		return nil
	}
}
