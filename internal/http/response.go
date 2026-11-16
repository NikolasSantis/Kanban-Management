package http

import (
	"kanban-management/internal/logger"

	"github.com/gofiber/fiber/v2"
)

func Success(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"success": true,
		"data":    data,
		"error":   nil,
	})
}

func Error(c *fiber.Ctx, status int, data any, errorMessage string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"data":    data,
		"error":   errorMessage,
	})
}

// ServerError registra o erro real (com request_id e user_id) e responde 500
// com uma mensagem genérica. err pode ser nil.
func ServerError(c *fiber.Ctx, err error, msg string) error {
	log := logger.FromCtx(c)

	if err != nil {
		log.Error(msg, "error", err)
	} else {
		log.Error(msg)
	}

	return Error(c, 500, fiber.Map{"error": msg}, msg)
}
