package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterProjectRoutes(app *fiber.App) {
	project := app.Group("/project", middleware.AuthMiddleware)

	project.Get("/", handler.MyProjects())
	project.Post("/", handler.CreateProject())
	project.Patch("/:id", handler.PatchProject())
	project.Delete("/:id", handler.DeleteProject())
}
