package routes

import (
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App) {
	// auth
	RegisterAuthRoutes(app)

	// users
	RegisterUsersRoutes(app)
}
