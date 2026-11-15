package main

import (
	"errors"
	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/routes"
	"kanban-management/internal/services"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
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

	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: !cfg.IsProduction(),
	}))

	routes.RegisterRoutes(app)

	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

// errorHandler responde erros não tratados (incluindo panics capturados pelo
// recover e rotas inexistentes) no envelope padrão, sem vazar detalhes internos.
func errorHandler(c *fiber.Ctx, err error) error {
	var fiberErr *fiber.Error

	if errors.As(err, &fiberErr) && fiberErr.Code < fiber.StatusInternalServerError {
		return http.Error(c, fiberErr.Code, fiber.Map{
			"error": fiberErr.Message,
		}, fiberErr.Message)
	}

	log.Printf("erro não tratado: %v", err)

	return http.Error(c, fiber.StatusInternalServerError, fiber.Map{
		"error": "Internal server error",
	}, "Internal server error")
}
