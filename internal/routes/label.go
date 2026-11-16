package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterLabelRoutes(app *fiber.App) {
	label := app.Group("/label", middleware.AuthMiddleware)

	// Labels de um board (:board_id)
	label.Get("/board/:board_id", handler.GetLabels())
	label.Post("/board/:board_id", handler.CreateLabel())

	// Label específica
	label.Patch("/:label_id", handler.PatchLabel())
	label.Delete("/:label_id", handler.DeleteLabel())
}
