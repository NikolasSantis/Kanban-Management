package app

import (
	"kanban-management/config"
	// A documentação gerada pelo swag (pasta docs/, versionada) se registra no import.
	_ "kanban-management/docs"

	"github.com/gofiber/fiber/v2"
	swagger "github.com/gofiber/swagger"
)

// registerSwagger serve a UI em /swagger/index.html e o JSON em /swagger/doc.json
// (desligável com SWAGGER_ENABLED=false).
func registerSwagger(app *fiber.App, cfg *config.Config) {
	if !cfg.SwaggerEnabled {
		return
	}

	app.Get("/swagger/*", swagger.HandlerDefault)
}
