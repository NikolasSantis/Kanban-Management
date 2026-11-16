package routes

import (
	"kanban-management/internal/handler"
	"kanban-management/internal/middleware"
	"kanban-management/internal/storage"

	"github.com/gofiber/fiber/v2"
)

func RegisterAttachmentRoutes(app *fiber.App, store storage.Storage) {
	attachment := app.Group("/attachment", middleware.AuthMiddleware)

	// Anexos de um card (:card_id)
	attachment.Get("/card/:card_id", handler.GetAttachments())
	attachment.Post("/card/:card_id", handler.UploadAttachment(store))
	attachment.Post("/card/:card_id/link", handler.CreateLinkAttachment())

	// Anexo específico
	attachment.Get("/:attachment_id/download", handler.DownloadAttachment(store))
	attachment.Delete("/:attachment_id", handler.DeleteAttachment(store))
}
