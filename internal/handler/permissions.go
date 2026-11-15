package handler

import (
	"errors"

	"kanban-management/internal/http"
	"kanban-management/internal/services"

	"github.com/gofiber/fiber/v2"
)

// respondAuthError traduz os erros de AuthorizeProject/AuthorizeWorkspace.
func respondAuthError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, services.ErrNotMember):
		return http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	case errors.Is(err, services.ErrForbidden):
		return http.Error(c, 403, fiber.Map{
			"error": "Forbidden",
		}, "Forbidden")
	default:
		return http.Error(c, 500, fiber.Map{
			"error": "Fail to check permissions",
		}, "Fail to check permissions")
	}
}
