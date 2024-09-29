package server

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Sourjaya/converse/app/auth"
	"github.com/Sourjaya/converse/app/templates/pages"
	"github.com/Sourjaya/converse/env"
	"github.com/Sourjaya/converse/middleware"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
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

func generateSecret() string {
	key := make([]byte, 32) // 32 bytes = 256 bits
	_, err := rand.Read(key)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func InitializeMiddleware(e *echo.Echo) {
	e.Use(echomiddleware.Logger())
	e.Use(echomiddleware.Recover())
	e.Use(echomiddleware.Secure())
	e.Use(middleware.WithRequestURL)
	//e.Use(middleware.ServerDelay)
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderContentLength, echo.HeaderAcceptEncoding, echo.HeaderXCSRFToken, echo.HeaderAuthorization, echo.HeaderCacheControl, echo.HeaderXRequestedWith},
		AllowMethods:     []string{http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPatch, http.MethodPost, http.MethodDelete, http.MethodOptions},
	}))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			ext := filepath.Ext(path)
			switch ext {
			case ".js":
				c.Response().Header().Set(echo.HeaderContentType, "application/javascript")
			case ".wasm":
				c.Response().Header().Set(echo.HeaderContentType, "application/wasm")
			}
			return next(c)
		}
	})
	e.Use(middleware.HXRedirectMiddleware)
	e.Any("/public/*", func(c echo.Context) error {
		if err := disableCache(staticDev())(c); err != nil {
			return err
		}
		return nil
	})
	e.GET("/env", func(c echo.Context) error {
		listenAddr := env.GetHTTPListenAddr()
		apiKey := env.GetDBApiURI()
		return c.JSON(http.StatusOK, map[string]string{
			"HTTP_LISTEN_ADDR": listenAddr,
			"DB_API_URI":       apiKey,
		})
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

func Start() {
	fmt.Println("getting started")
	sc.ListenAddr = env.GetHTTPListenAddr()
	hosts := map[string]*Host{}
	// App
	app := echo.New()
	InitializeMiddleware(app)
	app.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok && he.Code == http.StatusNotFound {
			auth.NotFoundHandler(c)
		} else {
			app.DefaultHTTPErrorHandler(err, c)
		}
	}
	store := sessions.NewCookieStore([]byte(generateSecret()))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   300, // 5 min
		HttpOnly: true,
		Secure:   true, // Only send over HTTPS
		SameSite: http.SameSiteLaxMode,
	}

	app.Use(session.Middleware(store))
	//app.Use(session.Middleware(sessions.NewCookieStore([]byte(generateSecret()))))
	hosts[fmt.Sprintf("app.localhost:%s", sc.ListenAddr)] = &Host{app}

	app.Static("/", "public")
	unguardedRoutes(app)

	// Landing site
	site := echo.New()
	InitializeMiddleware(site)
	site.HTTPErrorHandler = func(err error, c echo.Context) {
		if he, ok := err.(*echo.HTTPError); ok && he.Code == http.StatusNotFound {
			auth.NotFoundHandler(c)
		} else {
			site.DefaultHTTPErrorHandler(err, c)
		}
	}
	hosts[fmt.Sprintf("localhost:%s", sc.ListenAddr)] = &Host{site}

	site.GET("/", func(c echo.Context) error {
		loginURL := fmt.Sprintf("http://app.localhost:%s/login", sc.ListenAddr)
		component := pages.Index(loginURL)
		return component.Render(c.Request().Context(), c.Response().Writer)
	})

	site.Static("/", "public")

	// Server

	e := echo.New()
	InitializeMiddleware(e)
	e.Any("/*", func(c echo.Context) (err error) {
		req := c.Request()
		res := c.Response()
		host := hosts[req.Host]
		// fmt.Println("\n", status)
		if host != nil {
			host.Echo.ServeHTTP(res, req)
		} else {
			auth.NotFoundHandler(c)
		}
		return
	})

	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s", sc.ListenAddr)))
}

func unguardedRoutes(e *echo.Echo) {
	unguardedRoutes := e.Group("/")
	unguardedRoutes.Use(middleware.HXRedirectMiddleware)
	// unguardedRoutes.Use(services.GuestMiddleware)
	unguardedRoutes.GET("login", func(c echo.Context) error {
		registerURL := fmt.Sprintf("http://localhost:%s", sc.ListenAddr)
		component := pages.Login(pages.LoginPageData{}, registerURL)
		return component.Render(c.Request().Context(), c.Response().Writer)
	})
	unguardedRoutes.GET("redirectlogin", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/login")
	})
	unguardedRoutes.GET("register", func(c echo.Context) error {
		id := c.QueryParam("id")
		if id == "" {
			components := pages.Register(pages.RegisterPageData{})
			return components.Render(c.Request().Context(), c.Response().Writer)
		} else {
			return auth.HandleRedirectRegistration(c)
		}
	})
	//unguardedRoutes.GET("register?id=*", auth.HandleRedirectRegistration)
	//unguardedRoutes.GET("redirectregister",auth.HandleRegistration)
	// unguardedRoutes.GET("details", auth.HandlerTest)
	unguardedRoutes.POST("registration", auth.HandleRegistration)
	unguardedRoutes.POST("signup", auth.HandleSignup)
	unguardedRoutes.POST("view", auth.HandleShowPassword)
	unguardedRoutes.POST("viewC", auth.HandleShowConfirmPassword)
}
