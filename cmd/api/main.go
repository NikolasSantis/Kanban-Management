package main

import (
	"kanban-management/internal/database"
	"kanban-management/internal/routes"
	"kanban-management/internal/services"
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	if err := database.ConnectDB(); err != nil {
		log.Fatal(err)
	}

	services.RegisterEvents()

	app := fiber.New()

	routes.RegisterRoutes(app)

	app.Listen(":8000")
}
