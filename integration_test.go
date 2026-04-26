package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func newIntegrationServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	dir := t.TempDir()
	domain := "test.example.com"
	if err := os.MkdirAll(filepath.Join(dir, domain), 0755); err != nil {
		t.Fatal(err)
	}
	tmpl := `<result>{{.Email}}</result>`
	for _, name := range []string{"autodiscover.xml", "config-v1.1.xml"} {
		if err := os.WriteFile(filepath.Join(dir, domain, name), []byte(tmpl), 0644); err != nil {
			t.Fatal(err)
		}
	}

	e := echo.New()
	e.Use(middleware.BodyLimit(maxBodyBytes))
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			c.Request().URL.Path = strings.ToLower(c.Request().URL.Path)

			return next(c)
		}
	})

	h := &handler{templatesRoot: dir}
	e.GET("/health", h.health)
	e.POST("/autodiscover/autodiscover.xml", h.autodiscover)
	e.POST("/autodiscover.xml", h.autodiscover)
	e.GET("/.well-known/autoconfig/mail/config-v1.1.xml", h.autoconfig)
	e.GET("/mail/config-v1.1.xml", h.autoconfig)

	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)

	return srv, domain
}

func do(t *testing.T, srv *httptest.Server, method, path, host, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, srv.URL+path, bodyReader)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })

	return resp
}

func TestRouting(t *testing.T) {
	validBody := `<Autodiscover><Request><EMailAddress>user@example.com</EMailAddress></Request></Autodiscover>`

	t.Run("autodiscover path variants are case insensitive", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		for _, path := range []string{
			"/autodiscover/autodiscover.xml",
			"/Autodiscover/Autodiscover.xml",
			"/AUTODISCOVER/AUTODISCOVER.XML",
			"/AutoDiscover/autoDiscover.XmL",
		} {
			t.Run(path, func(t *testing.T) {
				resp := do(t, srv, http.MethodPost, path, domain, validBody)
				if resp.StatusCode != http.StatusOK {
					t.Errorf("status = %d, want 200", resp.StatusCode)
				}
			})
		}
	})

	t.Run("autodiscover without subdirectory", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		for _, path := range []string{
			"/autodiscover.xml",
			"/Autodiscover.xml",
			"/AUTODISCOVER.XML",
		} {
			t.Run(path, func(t *testing.T) {
				resp := do(t, srv, http.MethodPost, path, domain, validBody)
				if resp.StatusCode != http.StatusOK {
					t.Errorf("status = %d, want 200", resp.StatusCode)
				}
			})
		}
	})

	t.Run("autoconfig paths are case insensitive", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		for _, path := range []string{
			"/.well-known/autoconfig/mail/config-v1.1.xml",
			"/.well-known/AutoConfig/Mail/Config-v1.1.xml",
			"/.Well-Known/AUTOCONFIG/MAIL/CONFIG-V1.1.XML",
			"/mail/config-v1.1.xml",
			"/Mail/Config-v1.1.xml",
			"/MAIL/CONFIG-V1.1.XML",
		} {
			t.Run(path, func(t *testing.T) {
				resp := do(t, srv, http.MethodGet, path+"?emailaddress=user@example.com", domain, "")
				if resp.StatusCode != http.StatusOK {
					t.Errorf("status = %d, want 200", resp.StatusCode)
				}
			})
		}
	})

	t.Run("GET to autodiscover returns 405", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		resp := do(t, srv, http.MethodGet, "/autodiscover/autodiscover.xml", domain, "")
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
	})

	t.Run("POST to autoconfig returns 405", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		resp := do(t, srv, http.MethodPost, "/mail/config-v1.1.xml", domain, validBody)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want 405", resp.StatusCode)
		}
	})

	t.Run("unknown path returns 404", func(t *testing.T) {
		srv, _ := newIntegrationServer(t)
		resp := do(t, srv, http.MethodGet, "/not/a/real/path", "", "")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("health check", func(t *testing.T) {
		srv, _ := newIntegrationServer(t)
		resp := do(t, srv, http.MethodGet, "/health", "", "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})
}

func TestBodyLimit(t *testing.T) {
	t.Run("body within limit reaches handler", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		body := strings.Repeat("x", maxBodyBytes-1)
		resp := do(t, srv, http.MethodPost, "/autodiscover/autodiscover.xml", domain, body)
		// middleware passes it through; handler returns 400 for bad XML
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("body over limit returns 413", func(t *testing.T) {
		srv, domain := newIntegrationServer(t)
		body := strings.Repeat("x", maxBodyBytes+1)
		resp := do(t, srv, http.MethodPost, "/autodiscover/autodiscover.xml", domain, body)
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want 413", resp.StatusCode)
		}
	})
}
