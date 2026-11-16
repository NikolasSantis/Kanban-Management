package handler

import (
	"context"
	_ "kanban-management/internal/dto" // só para o swag resolver os tipos das anotações
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
//
//	@Summary		Verifica se a API e o banco estão no ar
//	@Description	Pública (sem autenticação). Faz ping no MongoDB com timeout de 2s.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	dto.HealthEnvelope
//	@Failure		503	{object}	dto.HealthErrorResponse	"Banco indisponível"
//	@Router			/health [get]
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
