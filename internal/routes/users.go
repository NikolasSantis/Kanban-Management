package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterUsersRoutes(app *fiber.App) {
	users := app.Group("/users", middleware.AuthMiddleware)

	users.Get("/:id", handler.GetUser())
	users.Patch("/:id", handler.PatchUser())
	users.Delete("/:id", handler.DeleteUser())
}
