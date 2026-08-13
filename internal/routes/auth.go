package routes

import (
	handlers "kanban-management/internal/handler"
	"kanban-management/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func RegisterAuthRoutes(app *fiber.App) {
	user := app.Group("/auth")

	user.Post("/login", handlers.Login())
	user.Post("/register", handlers.RegisterNewUser())
	user.Get("/me", middleware.AuthMiddleware, handlers.Me())
}
