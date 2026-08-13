package http

import "github.com/gofiber/fiber/v2"

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
