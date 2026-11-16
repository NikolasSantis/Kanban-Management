package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/logger"
	"kanban-management/internal/middleware"
	"kanban-management/internal/routes"
	"kanban-management/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// O logger ainda não foi configurado: usa o slog padrão.
		slog.Error("config inválida", "error", err)
		os.Exit(1)
	}

	logger.Setup(cfg)

	if err := database.ConnectDB(cfg); err != nil {
		slog.Error("falha ao conectar no MongoDB", "error", err)
		os.Exit(1)
	}

	services.RegisterEvents()

	app := fiber.New(fiber.Config{
		ErrorHandler: errorHandler,
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

	routes.RegisterRoutes(app)

	slog.Info("API iniciada", "port", cfg.Port, "env", cfg.AppEnv)

	if err := app.Listen(":" + cfg.Port); err != nil {
		slog.Error("falha ao iniciar o servidor", "error", err)
		os.Exit(1)
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

	return http.ServerError(c, err, "Internal server error")
}
