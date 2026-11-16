package middleware

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func newLoggedApp(buf *bytes.Buffer) *fiber.App {
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).SendString("erro")
		},
	})
	app.Use(requestid.New())
	app.Use(RequestLogger())
	app.Use(recover.New())

	app.Get("/ok/:id", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.Get("/panic", func(c *fiber.Ctx) error { panic("boom") })

	return app
}

func TestRequestLoggerOneLineWithoutSecrets(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	app := newLoggedApp(&buf)

	req := httptest.NewRequest("GET", "/ok/42", nil)
	req.Header.Set("Authorization", "Bearer segredo-super-secreto")
	req.Header.Set("X-Request-ID", "meu-id-123")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.Header.Get("X-Request-ID") != "meu-id-123" {
		t.Errorf("X-Request-ID não devolvido: %q", resp.Header.Get("X-Request-ID"))
	}

	out := buf.String()

	if n := strings.Count(strings.TrimSpace(out), "\n") + 1; n != 1 {
		t.Errorf("esperava 1 linha de log, veio %d: %s", n, out)
	}

	for _, want := range []string{"level=INFO", "method=GET", "route=/ok/:id", "status=200", "request_id=meu-id-123"} {
		if !strings.Contains(out, want) {
			t.Errorf("log sem %q: %s", want, out)
		}
	}

	for _, secret := range []string{"segredo-super-secreto", "Bearer", "Authorization"} {
		if strings.Contains(out, secret) {
			t.Errorf("log vazou %q: %s", secret, out)
		}
	}
}

func TestRequestLoggerPanicIsLoggedAs500(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	app := newLoggedApp(&buf)

	resp, err := app.Test(httptest.NewRequest("GET", "/panic", nil))
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 500 {
		t.Errorf("esperava 500, veio %d", resp.StatusCode)
	}

	if out := buf.String(); !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "status=500") {
		t.Errorf("panic deveria gerar ERROR com status=500: %s", out)
	}
}
