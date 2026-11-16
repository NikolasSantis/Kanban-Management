package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterBoardRoutes(app *fiber.App) {
	board := app.Group("/board", middleware.AuthMiddleware)

	// Boards de um projeto (:id = project id)
	board.Get("/project/:id", handler.GetBoards())
	board.Post("/project/:id", handler.CreateBoard())

	// Board específico
	board.Get("/:board_id", handler.GetBoard())
	board.Patch("/:board_id", handler.PatchBoard())
	board.Delete("/:board_id", handler.DeleteBoard())

	// Colunas de um board
	board.Post("/:board_id/columns", handler.AddColumn())
	board.Patch("/:board_id/columns/:column_id", handler.PatchColumn())
	board.Delete("/:board_id/columns/:column_id", handler.DeleteColumn())
}
