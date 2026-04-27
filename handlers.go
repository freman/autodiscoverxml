package main

import (
	"bytes"
	"fmt"
	"text/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/labstack/echo/v5"
)

var (
	httpErrBadRequest = echo.NewHTTPError(http.StatusBadRequest, http.StatusText(http.StatusBadRequest))
	httpErrTeapot     = echo.NewHTTPError(http.StatusTeapot, http.StatusText(http.StatusTeapot))
	httpErrInternal   = echo.NewHTTPError(http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
)

type handler struct {
	templatesRoot string
}

func (h *handler) autodiscover(c *echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil || len(body) == 0 {
		return httpErrBadRequest
	}

	raw, err := extractAutodiscoverEmail(body)
	if err != nil {
		slog.Info("autodiscover parse failure", "host", c.Request().Host, "err", err)

		return httpErrBadRequest
	}

	return h.parseAndRender(c, raw, "autodiscover.xml")
}

func (h *handler) autoconfig(c *echo.Context) error {
	raw := queryEmailAddress(c.Request())
	if raw == "" {
		return h.render(c, "config-v1.1.xml", nil)
	}

	return h.parseAndRender(c, raw, "config-v1.1.xml")
}

func queryEmailAddress(r *http.Request) string {
	for k, v := range r.URL.Query() {
		if strings.EqualFold(k, "emailaddress") && len(v) > 0 {
			return v[0]
		}
	}

	return ""
}

func (h *handler) parseAndRender(c *echo.Context, raw, tmplFile string) error {
	email, err := ParseEmail(raw)
	if err != nil {
		slog.Info("invalid email", "host", c.Request().Host, "err", err)

		return httpErrBadRequest
	}

	return h.render(c, tmplFile, &email)
}

func (h *handler) render(c *echo.Context, tmplFile string, email *Email) error {
	domain, err := safeDomain(c.Request().Host)
	if err != nil {
		slog.Info("invalid host header", "host", c.Request().Host, "err", err)

		return httpErrBadRequest
	}

	cfg, err := loadDomainConfig(filepath.Join(h.templatesRoot, domain, "config.json"))
	if err != nil {
		slog.Error("config load", "domain", domain, "err", err)

		return httpErrInternal
	}

	tmplPath := filepath.Join(h.templatesRoot, domain, tmplFile)
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		slog.Info("template not found", "domain", domain, "path", tmplPath)

		return httpErrTeapot
	}

	var buf bytes.Buffer
	data := struct {
		Email  *Email
		Config *DomainConfig
	}{Email: email, Config: cfg}

	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error("template execute", "domain", domain, "err", err)

		return httpErrInternal
	}

	emailDomain := ""
	if email != nil {
		emailDomain = email.Domain
	}

	slog.Info("served",
		"method", c.Request().Method,
		"path", c.Request().URL.Path,
		"host", c.Request().Host,
		"domain", domain,
		"email_domain", emailDomain,
		"status", http.StatusOK,
	)

	return c.Blob(http.StatusOK, "application/xml", buf.Bytes())
}

func (h *handler) health(c *echo.Context) error {
	return c.String(http.StatusOK, "ok")
}

func safeDomain(host string) (string, error) {
	domain := strings.TrimSpace(host)
	if strings.Contains(domain, ":") {
		var err error
		domain, _, err = net.SplitHostPort(domain)
		if err != nil {
			return "", fmt.Errorf("invalid host: %w", err)
		}
	}

	domain = strings.TrimSpace(domain)
	if domain == "" {
		return "", fmt.Errorf("empty host")
	}

	for _, ch := range domain {
		if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '.' && ch != '-' {
			return "", fmt.Errorf("invalid character %q in domain", ch)
		}
	}

	return domain, nil
}
