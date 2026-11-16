package routes

import (
	"kanban-management/internal/storage"

	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App, store storage.Storage) {
	RegisterHealthRoutes(app)
	RegisterAuthRoutes(app)
	RegisterUsersRoutes(app)
	RegisterWorkspaceRoutes(app)
	RegisterProjectRoutes(app)
	RegisterBoardRoutes(app)
	RegisterCardRoutes(app)
	RegisterLabelRoutes(app)
	RegisterCommentRoutes(app)
	RegisterAttachmentRoutes(app, store)
}
