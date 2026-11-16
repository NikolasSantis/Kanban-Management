package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kanban-management/config"
	"kanban-management/internal/app"
	"kanban-management/internal/database"
	"kanban-management/internal/healthcheck"
	"kanban-management/internal/logger"
	"kanban-management/internal/server"
)

const (
	shutdownTimeout   = 10 * time.Second
	disconnectTimeout = 5 * time.Second
)

func main() {
	healthFlag := flag.Bool("healthcheck", false, "consulta GET /health local e sai com 0 (ok) ou 1 (falha); usado pelo healthcheck do Docker")
	flag.Parse()

	// Antes de carregar config e banco: só olha a porta e consulta a própria API.
	if *healthFlag {
		if err := healthcheck.Check(healthcheck.URL(os.Getenv("PORT")), 3*time.Second); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		os.Exit(0)
	}

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

	listener, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		slog.Error("falha ao abrir a porta", "port", cfg.Port, "error", err)
		os.Exit(1)
	}

	// SIGINT (Ctrl+C) e SIGTERM (docker stop) encerram de forma graciosa.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("API iniciada", "port", cfg.Port, "env", cfg.AppEnv)

	exitCode := 0

	if err := server.Run(ctx, api, listener, shutdownTimeout); err != nil {
		slog.Error("falha no servidor", "error", err)
		exitCode = 1
	}

	disconnectCtx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
	defer cancel()

	if err := database.Disconnect(disconnectCtx); err != nil {
		slog.Error("falha ao desconectar do MongoDB", "error", err)
		exitCode = 1
	}

	slog.Info("API encerrada")

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
