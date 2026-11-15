package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAuthMiddlewareRejectsBadHeaders(t *testing.T) {
	app := fiber.New()
	app.Get("/", AuthMiddleware, func(c *fiber.Ctx) error { return c.SendStatus(200) })

	for name, header := range map[string]string{
		"sem header":    "",
		"sem esquema":   "abc",
		"esquema Basic": "Basic abc",
		"token demais":  "Bearer a b",
	} {
		req := httptest.NewRequest("GET", "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 401 {
			t.Errorf("%s: esperava 401, veio %d", name, resp.StatusCode)
		}
	}
}

func TestCurrentUserIDWithoutMiddleware(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if _, err := CurrentUserID(c); err == nil {
			t.Error("esperava erro")
		}
		return c.SendStatus(200)
	})
	if _, err := app.Test(httptest.NewRequest("GET", "/", nil)); err != nil {
		t.Fatal(err)
	}
}
