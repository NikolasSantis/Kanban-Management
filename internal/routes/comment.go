package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterCommentRoutes(app *fiber.App) {
	comment := app.Group("/comment", middleware.AuthMiddleware)

	// Comentários de um card (:card_id)
	comment.Get("/card/:card_id", handler.GetComments())
	comment.Post("/card/:card_id", handler.CreateComment())

	// Comentário específico
	comment.Patch("/:comment_id", handler.PatchComment())
	comment.Delete("/:comment_id", handler.DeleteComment())
}
