package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func healthApp(ping Pinger) *fiber.App {
	app := fiber.New()
	app.Get("/health", Health(ping))

	return app
}

func call(t *testing.T, app *fiber.App) (int, map[string]any) {
	t.Helper()

	resp, err := app.Test(httptest.NewRequest("GET", "/health", nil), -1)
	require.NoError(t, err)
	defer resp.Body.Close()

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	return resp.StatusCode, body
}

func TestHealth_DatabaseUp(t *testing.T) {
	status, body := call(t, healthApp(func(context.Context) error { return nil }))

	assert.Equal(t, 200, status)
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "ok", body["data"].(map[string]any)["status"])
}

func TestHealth_DatabaseDown(t *testing.T) {
	status, body := call(t, healthApp(func(context.Context) error { return errors.New("connection refused") }))

	assert.Equal(t, 503, status)
	assert.Equal(t, false, body["success"])
	assert.Equal(t, "unavailable", body["data"].(map[string]any)["status"])
	assert.NotContains(t, body["error"], "connection refused", "o erro real não vai para o cliente")
}

func TestHealth_PingTimeoutReturns503(t *testing.T) {
	previous := HealthTimeout
	HealthTimeout = 100 * time.Millisecond

	defer func() { HealthTimeout = previous }()

	start := time.Now()

	status, _ := call(t, healthApp(func(ctx context.Context) error {
		<-ctx.Done()

		return ctx.Err()
	}))

	assert.Equal(t, 503, status)
	assert.Less(t, time.Since(start), 2*time.Second, "o ping respeita o timeout")
}

func TestHealth_DoesNotRequireAuthentication(t *testing.T) {
	app := healthApp(func(context.Context) error { return nil })

	req := httptest.NewRequest("GET", "/health", nil)
	resp, err := app.Test(req, -1)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode, "sem header Authorization")
}
