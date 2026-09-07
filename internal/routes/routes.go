package routes

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App) {
	RegisterAuthRoutes(app)
	RegisterUsersRoutes(app)
	RegisterWorkspaceRoutes(app)
	RegisterProjectRoutes(app)
}
