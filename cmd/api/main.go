package main

import (
	"log/slog"
	"os"

	"kanban-management/config"
	"kanban-management/internal/app"
	"kanban-management/internal/database"
	"kanban-management/internal/logger"
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

	api := app.New(cfg)

	slog.Info("API iniciada", "port", cfg.Port, "env", cfg.AppEnv)

	if err := api.Listen(":" + cfg.Port); err != nil {
		slog.Error("falha ao iniciar o servidor", "error", err)
		os.Exit(1)
	}
}
