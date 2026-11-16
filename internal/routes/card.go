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

	// Calendário: cards do projeto com entrega no intervalo
	card.Get("/project/:project_id/due", handler.GetCardsDue())

	// Card específico
	card.Get("/:card_id", handler.GetCard())
	card.Patch("/:card_id", handler.PatchCard())
	card.Patch("/:card_id/move", handler.MoveCard())
	card.Post("/:card_id/complete", handler.CompleteCard())
	card.Post("/:card_id/reopen", handler.ReopenCard())
	card.Delete("/:card_id", handler.DeleteCard())
}
