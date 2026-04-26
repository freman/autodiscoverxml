package main

import (
	"flag"
	"log/slog"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const maxBodyBytes = 16 * 1024

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	templates := flag.String("templates", "", "path to template root directory")
	flag.Parse()

	if *templates == "" {
		slog.Error("--templates is required")
		os.Exit(1)
	}

	e := echo.New()

	e.Use(middleware.BodyLimit(maxBodyBytes))

	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Request().URL.Path = strings.ToLower(c.Request().URL.Path)

			return next(c)
		}
	})

	h := &handler{templatesRoot: *templates}

	e.GET("/health", h.health)

	e.POST("/autodiscover/autodiscover.xml", h.autodiscover)
	e.POST("/autodiscover.xml", h.autodiscover)
	e.GET("/.well-known/autoconfig/mail/config-v1.1.xml", h.autoconfig)
	e.GET("/mail/config-v1.1.xml", h.autoconfig)

	slog.Info("starting", "addr", *addr, "templates", *templates)
	if err := e.Start(*addr); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
