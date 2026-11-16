package handler

import (
	"context"
	"log/slog"
	"time"

	"kanban-management/internal/http"

	"github.com/gofiber/fiber/v2"
)

// HealthTimeout é o tempo máximo do ping no banco.
var HealthTimeout = 2 * time.Second

// Pinger verifica se o banco responde (em produção: o Ping do cliente do Mongo).
type Pinger func(ctx context.Context) error

// Health responde 200 {"status":"ok"} quando o banco responde ao ping (timeout de
// 2s) e 503 {"status":"unavailable"} quando não. É pública (sem autenticação).
func Health(ping Pinger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), HealthTimeout)
		defer cancel()

		if err := ping(ctx); err != nil {
			slog.Warn("health check failed", "error", err)

			return http.Error(c, 503, fiber.Map{"status": "unavailable"}, "Database unavailable")
		}

		return http.Success(c, 200, fiber.Map{"status": "ok"})
	}
}
