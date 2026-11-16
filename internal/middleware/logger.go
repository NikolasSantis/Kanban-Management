package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// RequestLogger escreve uma linha de log por request. Nunca loga body nem
// headers (Authorization, cookies etc.). Deve ficar depois do requestid e
// ANTES do recover, para enxergar o erro de um panic já convertido.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		chainErr := c.Next()

		// Erros retornados pelos handlers só viram resposta no ErrorHandler,
		// então ele é chamado aqui para o status ficar correto no log.
		if chainErr != nil {
			if err := c.App().Config().ErrorHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}

		status := c.Response().StatusCode()

		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}

		attrs := []any{
			"method", c.Method(),
			"route", c.Route().Path,
			"url", c.OriginalURL(),
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.IP(),
			"bytes", len(c.Response().Body()),
		}

		if requestID, ok := c.Locals("requestid").(string); ok {
			attrs = append(attrs, "request_id", requestID)
		}

		if userID, ok := c.Locals("user_id").(string); ok {
			attrs = append(attrs, "user_id", userID)
		}

		slog.Log(c.Context(), level, "request", attrs...)

		return nil
	}
}
