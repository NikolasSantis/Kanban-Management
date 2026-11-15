package main

import (
	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/routes"
	"kanban-management/internal/services"
	"log"

	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config inválida: %v", err)
	}

	if err := database.ConnectDB(cfg); err != nil {
		log.Fatal(err)
	}

	services.RegisterEvents()

	app := fiber.New()

	routes.RegisterRoutes(app)

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
