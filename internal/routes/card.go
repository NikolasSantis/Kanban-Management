package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterCardRoutes(app *fiber.App) {
	card := app.Group("/card", middleware.AuthMiddleware)

	// Cards de um board (:board_id)
	card.Get("/board/:board_id", handler.GetCards())
	card.Post("/board/:board_id", handler.CreateCard())

	// Card específico
	card.Get("/:card_id", handler.GetCard())
	card.Patch("/:card_id", handler.PatchCard())
	card.Delete("/:card_id", handler.DeleteCard())
}
