package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterWorkspaceRoutes(app *fiber.App) {
	workspace := app.Group("/workspace", middleware.AuthMiddleware)

	workspace.Get("/", handler.GetWorkspaces())
	workspace.Post("/", handler.CreateWorkspace())
	workspace.Patch("/:id", handler.PatchWorkspace())
	workspace.Delete("/:id", handler.DeleteWorkspace())
}
