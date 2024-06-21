package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Sourjaya/converse/app/templates/pages"
	"github.com/Sourjaya/converse/middleware"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func Load_Env() {
	if err := godotenv.Load(".env"); err != nil {
		log.Fatal(err)
	}
}

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
	e := echo.New()
	InitializeMiddleware(e)
	e.Any("/public/*", func(c echo.Context) error {
		if err := disableCache(staticDev())(c); err != nil {
			return err
		}
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unguardedRoutes(e, &ctx)
	//Load_Env()
	listenAddr := os.Getenv("HTTP_LISTEN_ADDR")

	e.Static("./app/assets", "css")
	e.Static("./public/assets", "css")
	//e.Static("/static", "static")
	e.Logger.Fatal(e.Start(listenAddr))
}
func unguardedRoutes(e *echo.Echo, ctx *context.Context) {
	unguardedRoutes := e.Group("/")
	//unguardedRoutes.Use(services.GuestMiddleware)
	unguardedRoutes.GET("", func(c echo.Context) error {
		component := pages.Index()
		return component.Render(*ctx, c.Response().Writer)
	})
	unguardedRoutes.GET("login", func(c echo.Context) error {
		component := pages.Login()
		return component.Render(*ctx, c.Response().Writer)
	})
	unguardedRoutes.GET("redirectlogin", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/login")
	})
}
