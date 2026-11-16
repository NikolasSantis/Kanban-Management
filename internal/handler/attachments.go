package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	nethttp "net/http"
	"time"

	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/services"
	"kanban-management/internal/storage"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type linkRequest struct {
	Name *string `json:"name"`
	URL  *string `json:"url"`
}

func GetAttachments() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, _, _, handled, err := loadCardForUser(c, ctx, services.ActionCardView)

		if handled {
			return err
		}

		cursor, err := database.GetCollection("attachments").Find(ctx, bson.M{
			"card_id": *card.ID,
			"$or":     services.NotDeleted(),
		}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))

		if err != nil {
			return http.ServerError(c, err, "Fail to get attachments")
		}

		defer cursor.Close(ctx)

		attachments := []models.Attachment{}

		if err := cursor.All(ctx, &attachments); err != nil {
			return http.ServerError(c, err, "Fail to decode attachments")
		}

		return http.Success(c, 200, models.ToAttachmentResponses(attachments))
	}
}

// attachmentLimitReached responde 409 se o card já tem o máximo de anexos.
func attachmentLimitReached(c *fiber.Ctx, ctx context.Context, cardID bson.ObjectID) (bool, error) {
	count, err := services.ActiveAttachmentCount(ctx, cardID)

	if err != nil {
		return true, http.ServerError(c, err, "Fail to count attachments")
	}

	if count >= services.MaxAttachmentsPerCard {
		return true, http.Error(c, 409, fiber.Map{
			"error": "A card can have at most 20 attachments",
		}, "A card can have at most 20 attachments")
	}

	return false, nil
}

// UploadAttachment recebe multipart/form-data com o campo "file". O storage é
// injetado (local em produção, memória nos testes).
func UploadAttachment(store storage.Storage) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, userID, _, handled, err := loadCardForUser(c, ctx, services.ActionAttachmentCreate)

		if handled {
			return err
		}

		if limited, err := attachmentLimitReached(c, ctx, *card.ID); limited {
			return err
		}

		header, err := c.FormFile("file")

		if err != nil {
			return invalidCardResponse(c, "Field file (multipart/form-data) is required")
		}

		maxBytes := int64(config.Get().AttachmentMaxSizeMB) * 1024 * 1024

		if header.Size > maxBytes {
			return http.Error(c, 413, fiber.Map{
				"error": "File too large",
			}, "File too large")
		}

		if header.Size == 0 {
			return invalidCardResponse(c, "File is empty")
		}

		file, err := header.Open()

		if err != nil {
			return http.ServerError(c, err, "Fail to read uploaded file")
		}

		defer file.Close()

		// O tipo vem do CONTEÚDO (primeiros 512 bytes), não da extensão.
		head := make([]byte, 512)
		n, err := io.ReadFull(file, head)

		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
			return http.ServerError(c, err, "Fail to read uploaded file")
		}

		contentType := nethttp.DetectContentType(head[:n])

		if !services.IsAllowedContentType(contentType) {
			return http.Error(c, 400, fiber.Map{
				"error": "File type not allowed",
			}, "File type not allowed")
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return http.ServerError(c, err, "Fail to read uploaded file")
		}

		attachmentID := bson.NewObjectID()
		fileName := services.SanitizeFileName(header.Filename)
		key := services.BuildStorageKey(*card.ID, attachmentID, header.Filename)

		if err := store.Save(ctx, key, file, header.Size, contentType); err != nil {
			return http.ServerError(c, err, "Fail to store file")
		}

		attachment := models.Attachment{
			ID:                &attachmentID,
			ProjectID:         card.ProjectID,
			BoardID:           card.BoardID,
			CardID:            card.ID,
			UploadedBy:        &userID,
			Kind:              models.AttachmentKindFile,
			FileName:          fileName,
			OriginalName:      services.DisplayName(header.Filename),
			ContentType:       contentType,
			ClientContentType: header.Header.Get("Content-Type"),
			SizeBytes:         header.Size,
			StorageKey:        key,
			CreatedAt:         time.Now(),
		}

		if _, err := database.GetCollection("attachments").InsertOne(ctx, attachment); err != nil {
			// Não deixa arquivo órfão no storage.
			if delErr := store.Delete(ctx, key); delErr != nil {
				slog.Warn("fail to remove orphan file after insert error", "key", key, "error", delErr)
			}

			return http.ServerError(c, err, "Attachment not created")
		}

		slog.Info("attachment.created", "attachment_id", attachmentID.Hex(), "card_id", card.ID.Hex(), "kind", "file", "user_id", userID.Hex())

		return http.Success(c, 201, models.ToAttachmentResponse(attachment))
	}
}

// CreateLinkAttachment cria um anexo "por link" (sem arquivo).
func CreateLinkAttachment() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, userID, _, handled, err := loadCardForUser(c, ctx, services.ActionAttachmentCreate)

		if handled {
			return err
		}

		var request linkRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid body")
		}

		if request.Name == nil || request.URL == nil {
			return invalidCardResponse(c, "name and url are required")
		}

		name, linkURL, err := services.ValidateLink(*request.Name, *request.URL)

		if err != nil {
			return invalidCardResponse(c, "Invalid link: name must have 1 to 100 characters and url must be http or https")
		}

		if limited, err := attachmentLimitReached(c, ctx, *card.ID); limited {
			return err
		}

		attachmentID := bson.NewObjectID()

		attachment := models.Attachment{
			ID:           &attachmentID,
			ProjectID:    card.ProjectID,
			BoardID:      card.BoardID,
			CardID:       card.ID,
			UploadedBy:   &userID,
			Kind:         models.AttachmentKindLink,
			FileName:     name,
			OriginalName: name,
			URL:          linkURL,
			CreatedAt:    time.Now(),
		}

		if _, err := database.GetCollection("attachments").InsertOne(ctx, attachment); err != nil {
			return http.ServerError(c, err, "Attachment not created")
		}

		slog.Info("attachment.created", "attachment_id", attachmentID.Hex(), "card_id", card.ID.Hex(), "kind", "link", "user_id", userID.Hex())

		return http.Success(c, 201, models.ToAttachmentResponse(attachment))
	}
}

// loadAttachmentForUser carrega o anexo ativo (de um card ativo) e confere que
// o usuário é membro do projeto. handled=true: a resposta já foi escrita.
func loadAttachmentForUser(c *fiber.Ctx, ctx context.Context) (attachment *models.Attachment, userID bson.ObjectID, role string, handled bool, err error) {
	attachmentID, err := bson.ObjectIDFromHex(c.Params("attachment_id"))

	if err != nil {
		return nil, userID, "", true, invalidCardResponse(c, "Invalid attachment id")
	}

	userID, err = middleware.CurrentUserID(c)

	if err != nil {
		return nil, userID, "", true, http.Error(c, 401, fiber.Map{
			"error": "Fail to get user auth",
		}, "Fail to get user auth")
	}

	attachment, err = services.FindActiveAttachment(ctx, attachmentID)

	if err != nil || attachment.ProjectID == nil || attachment.CardID == nil {
		return nil, userID, "", true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	if _, err := services.FindActiveCard(ctx, *attachment.CardID); err != nil {
		return nil, userID, "", true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	role, err = services.AuthorizeProject(ctx, userID, *attachment.ProjectID, services.ActionCardView)

	if err != nil {
		return nil, userID, role, true, respondAuthError(c, err)
	}

	return attachment, userID, role, false, nil
}

// DownloadAttachment faz o stream do arquivo passando pela API (com permissão).
func DownloadAttachment(store storage.Storage) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		attachment, _, _, handled, err := loadAttachmentForUser(c, ctx)

		if handled {
			return err
		}

		if attachment.Kind != models.AttachmentKindFile {
			return invalidCardResponse(c, "Only file attachments can be downloaded")
		}

		reader, err := store.Open(ctx, attachment.StorageKey)

		if errors.Is(err, storage.ErrNotFound) {
			return http.Error(c, 404, fiber.Map{
				"error": "File not found",
			}, "File not found")
		}

		if err != nil {
			return http.ServerError(c, err, "Fail to open file")
		}

		c.Set(fiber.HeaderContentType, attachment.ContentType)
		c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{"filename": attachment.OriginalName}))
		c.Set("X-Content-Type-Options", "nosniff")

		// O fasthttp fecha o reader ao terminar de enviar.
		return c.SendStream(reader, int(attachment.SizeBytes))
	}
}

// DeleteAttachment: quem enviou ou attachment:moderate. Soft delete no Mongo e
// remoção do arquivo no storage (falha ali só vira WARN).
func DeleteAttachment(store storage.Storage) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		attachment, userID, role, handled, err := loadAttachmentForUser(c, ctx)

		if handled {
			return err
		}

		isUploader := attachment.UploadedBy != nil && *attachment.UploadedBy == userID

		if !isUploader && !services.Can(role, services.ActionAttachmentModerate) {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		result, err := database.GetCollection("attachments").UpdateByID(ctx, *attachment.ID, bson.M{
			"$set": bson.M{"deleted_at": time.Now()},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete attachment")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No attachment modified")
		}

		if attachment.Kind == models.AttachmentKindFile && attachment.StorageKey != "" {
			if err := store.Delete(ctx, attachment.StorageKey); err != nil {
				slog.Warn("fail to remove attachment file", "attachment_id", attachment.ID.Hex(), "key", attachment.StorageKey, "error", err)
			}
		}

		slog.Info("attachment.deleted", "attachment_id", attachment.ID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
