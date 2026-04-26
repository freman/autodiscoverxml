package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestSafeDomain(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		want    string
		wantErr bool
	}{
		{"plain domain", "example.com", "example.com", false},
		{"with port", "example.com:8080", "example.com", false},
		{"https port", "example.com:443", "example.com", false},
		{"trailing empty port", "example.com:", "example.com", false},
		{"subdomain", "autodiscover.example.com", "autodiscover.example.com", false},
		{"leading and trailing spaces", "  example.com  ", "example.com", false},
		{"spaces with port", "  example.com:8080  ", "example.com", false},
		{"ip address", "192.168.1.1", "192.168.1.1", false},
		{"ip with port", "192.168.1.1:8080", "192.168.1.1", false},
		{"localhost", "localhost", "localhost", false},
		{"localhost with port", "localhost:8080", "localhost", false},
		{"non-numeric port is fine", "example.com:notaport", "example.com", false},

		{"empty string", "", "", true},
		{"whitespace only", "    ", "", true},
		{"path traversal with slashes", "example.com/../../etc/passwd", "", true},
		{"backslash betrayal", `example.com\evil`, "", true},
		{"null byte ninja", "example.com\x00", "", true},
		{"crlf injection", "example.com\r\nX-Evil: injected", "", true},
		{"at sign ambush", "user@example.com", "", true},
		{"underscore", "example_com", "", true},
		{"emoji chaos", "🔥.example.com", "", true},
		{"ipv6 bracket host", "[::1]:8080", "", true},
		{"port only no host", ":8080", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := safeDomain(tt.host)
			if (err != nil) != tt.wantErr {
				t.Fatalf("safeDomain(%q) error = %v, wantErr %v", tt.host, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("safeDomain(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func newTestHandler(t *testing.T) (*handler, string) {
	t.Helper()
	dir := t.TempDir()
	domain := "test.example.com"
	if err := os.MkdirAll(filepath.Join(dir, domain), 0755); err != nil {
		t.Fatal(err)
	}
	templates := map[string]string{
		"autodiscover.xml": `<result><email>{{.Email}}</email><user>{{.Email.User}}</user><domain>{{.Email.Domain}}</domain></result>`,
		"config-v1.1.xml":  `<result>{{if .Email}}<email>{{.Email}}</email><user>{{.Email.User}}</user><domain>{{.Email.Domain}}</domain>{{else}}<mozilla/>{{end}}</result>`,
	}
	for name, content := range templates {
		if err := os.WriteFile(filepath.Join(dir, domain, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return &handler{templatesRoot: dir}, domain
}

func echoErr(t *testing.T, err error) *echo.HTTPError {
	t.Helper()
	var he *echo.HTTPError
	if !errors.As(err, &he) {
		t.Fatalf("expected *echo.HTTPError, got %T: %v", err, err)
	}

	return he
}

func TestAutodiscoverHandler(t *testing.T) {
	validBody := `<Autodiscover><Request><EMailAddress>user@example.com</EMailAddress></Request></Autodiscover>`

	t.Run("valid request renders template", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/autodiscover/autodiscover.xml", strings.NewReader(validBody))
		req.Host = domain
		rec := httptest.NewRecorder()
		if err := h.autodiscover(echo.NewContext(req, rec)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/xml" {
			t.Errorf("Content-Type = %q, want application/xml", ct)
		}
		if !strings.Contains(rec.Body.String(), "user@example.com") {
			t.Errorf("body does not contain email: %s", rec.Body.String())
		}
	})

	t.Run("email user and domain available in template", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validBody))
		req.Host = domain
		rec := httptest.NewRecorder()
		if err := h.autodiscover(echo.NewContext(req, rec)); err != nil {
			t.Fatal(err)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "<user>user</user>") {
			t.Errorf("body missing user part: %s", body)
		}
		if !strings.Contains(body, "<domain>example.com</domain>") {
			t.Errorf("body missing domain part: %s", body)
		}
	})

	t.Run("empty body returns 400", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		req.Host = domain
		err := h.autodiscover(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("invalid xml returns 400", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("<unclosed"))
		req.Host = domain
		err := h.autodiscover(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("xml with no email returns 400", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("<Autodiscover><Request></Request></Autodiscover>"))
		req.Host = domain
		err := h.autodiscover(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("invalid email in xml returns 400", func(t *testing.T) {
		h, domain := newTestHandler(t)
		body := `<Autodiscover><Request><EMailAddress>notanemail</EMailAddress></Request></Autodiscover>`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		req.Host = domain
		err := h.autodiscover(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("screwy host header returns 400", func(t *testing.T) {
		h, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validBody))
		req.Host = "../../etc/passwd"
		err := h.autodiscover(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("unknown domain returns 418", func(t *testing.T) {
		h, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validBody))
		req.Host = "nobody.knows.me"
		err := h.autodiscover(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusTeapot {
			t.Errorf("want 418, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("all autodiscover xml variants accepted", func(t *testing.T) {
		variants := []struct {
			name string
			body string
		}{
			{"soap", `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:a="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006"><soap:Body><a:Autodiscover><a:Request><a:EMailAddress>user@example.com</a:EMailAddress></a:Request></a:Autodiscover></soap:Body></soap:Envelope>`},
			{"namespaced", `<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006"><Request><EMailAddress>user@example.com</EMailAddress></Request></Autodiscover>`},
			{"bare EmailAddress", `<Autodiscover><Request><EmailAddress>user@example.com</EmailAddress></Request></Autodiscover>`},
			{"request only", `<Request><EMailAddress>user@example.com</EMailAddress></Request>`},
			{"all caps", `<AUTODISCOVER><REQUEST><EMAILADDRESS>user@example.com</EMAILADDRESS></REQUEST></AUTODISCOVER>`},
		}
		h, domain := newTestHandler(t)
		for _, v := range variants {
			t.Run(v.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(v.body))
				req.Host = domain
				rec := httptest.NewRecorder()
				if err := h.autodiscover(echo.NewContext(req, rec)); err != nil {
					t.Fatalf("variant %q failed: %v", v.name, err)
				}
			})
		}
	})
}

func TestAutoconfigHandler(t *testing.T) {
	t.Run("valid request renders template", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/.well-known/autoconfig/mail/config-v1.1.xml?emailaddress=user@example.com", nil)
		req.Host = domain
		rec := httptest.NewRecorder()
		if err := h.autoconfig(echo.NewContext(req, rec)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "user@example.com") {
			t.Errorf("body does not contain email: %s", rec.Body.String())
		}
	})

	t.Run("missing emailaddress param renders mozilla tokens", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/mail/config-v1.1.xml", nil)
		req.Host = domain
		rec := httptest.NewRecorder()
		if err := h.autoconfig(echo.NewContext(req, rec)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "<mozilla/>") {
			t.Errorf("body should have fallen through to mozilla branch: %s", rec.Body.String())
		}
	})

	t.Run("invalid email param returns 400", func(t *testing.T) {
		h, domain := newTestHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/mail/config-v1.1.xml?emailaddress=notanemail", nil)
		req.Host = domain
		err := h.autoconfig(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("unknown domain returns 418", func(t *testing.T) {
		h, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/mail/config-v1.1.xml?emailaddress=user@example.com", nil)
		req.Host = "nobody.knows.me"
		err := h.autoconfig(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusTeapot {
			t.Errorf("want 418, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("screwy host header returns 400", func(t *testing.T) {
		h, _ := newTestHandler(t)
		req := httptest.NewRequest(http.MethodGet, "/mail/config-v1.1.xml?emailaddress=user@example.com", nil)
		req.Host = "example.com\r\nX-Evil: injected"
		err := h.autoconfig(echo.NewContext(req, httptest.NewRecorder()))
		if echoErr(t, err).Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", echoErr(t, err).Code)
		}
	})

	t.Run("emailaddress param is case insensitive", func(t *testing.T) {
		h, domain := newTestHandler(t)
		for _, param := range []string{"emailaddress", "EmailAddress", "EMAILADDRESS", "Emailaddress"} {
			t.Run(param, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/mail/config-v1.1.xml?"+param+"=user@example.com", nil)
				req.Host = domain
				rec := httptest.NewRecorder()
				if err := h.autoconfig(echo.NewContext(req, rec)); err != nil {
					t.Fatalf("param %q: unexpected error: %v", param, err)
				}
			})
		}
	})
}

func TestHealthHandler(t *testing.T) {
	h := &handler{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	if err := h.health(echo.NewContext(req, rec)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("body = %q, want \"ok\"", rec.Body.String())
	}
}
