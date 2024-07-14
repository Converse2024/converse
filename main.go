package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Sourjaya/converse/app/auth"
	"github.com/Sourjaya/converse/app/templates/pages"
	"github.com/Sourjaya/converse/env"
	"github.com/Sourjaya/converse/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

type Host struct {
	Echo *echo.Echo
}

type ServerConfig struct {
	ListenAddr string
}

var sc ServerConfig

func InitializeMiddleware(e *echo.Echo) {
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(middleware.WithRequestURL)
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderContentLength, echo.HeaderAcceptEncoding, echo.HeaderXCSRFToken, echo.HeaderAuthorization, echo.HeaderCacheControl, echo.HeaderXRequestedWith},
		AllowMethods:     []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete, http.MethodOptions},
	}))
	e.Use(middleware.HXRedirectMiddleware)
	e.Any("/public/*", func(c echo.Context) error {
		if err := disableCache(staticDev())(c); err != nil {
			return err
		}
		return nil
	})
}

func staticDev() echo.HandlerFunc {
	return echo.WrapHandler(http.StripPrefix("/public/", http.FileServerFS(os.DirFS("public"))))
}

func disableCache(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "no-store")
		return next(c)
	}
}

// router.Handle("/public/*", disableCache(staticDev()))
func main() {
	fmt.Println("getting started")
	sc.ListenAddr = env.GetHTTPListenAddr()
	hosts := map[string]*Host{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// App
	app := echo.New()
	InitializeMiddleware(app)

	hosts[fmt.Sprintf("app.localhost:%s", sc.ListenAddr)] = &Host{app}

	app.Static("/", "app/assets")
	app.Static("/", "public/assets")
	unguardedRoutes(app, &ctx)

	// Landing site
	site := echo.New()
	InitializeMiddleware(site)
	hosts[fmt.Sprintf("localhost:%s", sc.ListenAddr)] = &Host{site}

	site.GET("/", func(c echo.Context) error {
		loginURL := fmt.Sprintf("http://app.localhost:%s/login", sc.ListenAddr)
		component := pages.Index(loginURL)
		return component.Render(ctx, c.Response().Writer)
	})

	// Server
	e := echo.New()
	InitializeMiddleware(e)
	e.Any("/*", func(c echo.Context) (err error) {
		req := c.Request()
		res := c.Response()
		host := hosts[req.Host]

		if host == nil {
			err = echo.ErrNotFound
		} else {
			host.Echo.ServeHTTP(res, req)
		}

		return
	})

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", sc.ListenAddr)))
}

func unguardedRoutes(e *echo.Echo, ctx *context.Context) {
	unguardedRoutes := e.Group("/")
	unguardedRoutes.Use(middleware.HXRedirectMiddleware)
	// unguardedRoutes.Use(services.GuestMiddleware)
	unguardedRoutes.GET("login", func(c echo.Context) error {
		registerURL := fmt.Sprintf("http://localhost:%s", sc.ListenAddr)
		component := pages.Login(pages.LoginPageData{}, registerURL)
		return component.Render(*ctx, c.Response().Writer)
	})
	unguardedRoutes.GET("redirectlogin", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/login")
	})
	unguardedRoutes.GET("register", func(c echo.Context) error {
		components := pages.Register(pages.RegisterPageData{})
		return components.Render(*ctx, c.Response().Writer)
	})
	unguardedRoutes.POST("registration", auth.HandleRegistration)
	//unguardedRoutes.GET("input", auth.HandleGetEmail)

	// unguardedRoutes.GET("otp", func(c echo.Context) error {
	// 	components := pages.Otp()
	// 	return components.Render(*ctx, c.Response().Writer)
	// })
}
