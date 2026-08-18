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

	workspace.Get("/member", handler.GetMyWorkspaces())
	workspace.Get("/member/:id", handler.GetMembersOnWorkspace())
	workspace.Post("/member/:id", handler.AddMemberToWorkspace())
	workspace.Patch("/member/:id", handler.PatchWorkspaceMember())
	workspace.Delete("/member/:id", handler.DeleteWorkspaceMember())
}
