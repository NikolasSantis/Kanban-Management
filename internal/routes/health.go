package routes

import (
	"context"

	"kanban-management/internal/database"
	"kanban-management/internal/handler"

	"github.com/gofiber/fiber/v2"
)

// RegisterHealthRoutes registra GET /health (público, sem autenticação).
func RegisterHealthRoutes(app *fiber.App) {
	app.Get("/health", handler.Health(func(ctx context.Context) error {
		return database.DB.Client().Ping(ctx, nil)
	}))
}
