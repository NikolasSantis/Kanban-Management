package http

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func TestServerErrorLogsRealErrorWithRequestID(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))

	app := fiber.New()
	app.Use(requestid.New())
	app.Get("/", func(c *fiber.Ctx) error {
		return ServerError(c, errors.New("connection reset"), "Error to update")
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "rid-1")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 500 {
		t.Fatalf("esperava 500, veio %d", resp.StatusCode)
	}

	out := buf.String()
	for _, want := range []string{"level=ERROR", `msg="Error to update"`, `error="connection reset"`, "request_id=rid-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("log sem %q: %s", want, out)
		}
	}
}

func TestServerErrorDoesNotLeakErrorToClient(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return ServerError(c, errors.New("segredo interno"), "Falhou")
	})

	resp, _ := app.Test(httptest.NewRequest("GET", "/", nil))
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)

	if strings.Contains(buf.String(), "segredo interno") {
		t.Errorf("resposta vazou o erro: %s", buf.String())
	}
}
