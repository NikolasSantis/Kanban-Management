// Package app monta o Fiber (middlewares, eventos e rotas) sem conectar no
// banco nem chamar Listen, para ser reutilizado pelo main e pelos testes.
package app

import (
	"errors"
	"fmt"
	"runtime/debug"

	"kanban-management/config"
	"kanban-management/internal/http"
	"kanban-management/internal/logger"
	"kanban-management/internal/middleware"
	"kanban-management/internal/routes"
	"kanban-management/internal/services"
	"kanban-management/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

// New cria o app. O banco (database.ConnectDB) precisa estar conectado antes
// das requests.
func New(cfg *config.Config) *fiber.App {
	services.RegisterEvents()

	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
		// Um pouco acima do limite de anexo, para o handler responder 413 com mensagem.
		BodyLimit: (cfg.AttachmentMaxSizeMB + 1) * 1024 * 1024,
		// O banner do Fiber não é JSON; com LOG_FORMAT=json toda linha precisa ser.
		DisableStartupMessage: cfg.LogFormat == "json",
	})

	// Ordem: requestid → RequestLogger → recover. O recover fica depois do
	// logger para que o panic vire erro retornado e o log de acesso registre 500.
	app.Use(requestid.New())
	app.Use(middleware.RequestLogger())
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, e any) {
			logger.FromCtx(c).Error("panic recovered", "panic", fmt.Sprint(e), "stack", string(debug.Stack()))
		},
	}))

	routes.RegisterRoutes(app, storage.New(cfg))

	return app
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

	return http.ServerError(c, err, "Internal server error")
}
