// Package logger configura o slog da aplicação. Não importa internal/http para
// evitar import cíclico (http.ServerError usa logger.FromCtx).
package logger

import (
	"io"
	"log/slog"
	"os"

	"kanban-management/config"

	"github.com/gofiber/fiber/v2"
)

// Setup cria o logger conforme LOG_LEVEL/LOG_FORMAT e o define como padrão do slog.
func Setup(cfg *config.Config) *slog.Logger {
	return setup(cfg, os.Stderr)
}

func setup(cfg *config.Config, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}

	var handler slog.Handler

	if cfg.LogFormat == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FromCtx retorna o logger padrão com request_id e user_id (quando existirem)
// já anexados.
func FromCtx(c *fiber.Ctx) *slog.Logger {
	logger := slog.Default()

	if requestID, ok := c.Locals("requestid").(string); ok && requestID != "" {
		logger = logger.With("request_id", requestID)
	}

	if userID, ok := c.Locals("user_id").(string); ok && userID != "" {
		logger = logger.With("user_id", userID)
	}

	return logger
}
