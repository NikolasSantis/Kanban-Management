package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	_ "kanban-management/internal/dto" // só para o swag resolver os tipos das anotações
	"kanban-management/internal/filters"
	"kanban-management/internal/pagination"
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
)

type linkRequest struct {
	Name *string `json:"name"`
	URL  *string `json:"url"`
}

// @Summary	Lista os anexos de um card
// @Tags		attachments
// @Accept		json
// @Produce	json
// @Param		card_id	path		string	true	"ID do card (ObjectID)"
// @Param		page	query		int		false	"Página (padrão 1)"							default(1)	minimum(1)
// @Param		limit	query		int		false	"Itens por página (padrão 20, máximo 100)"	default(20)	minimum(1)	maximum(100)
// @Param		kind	query		string	false	"file ou link"								Enums(file, link)
// @Success	200		{object}	dto.PageAttachmentsEnvelope
// @Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure	401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure	404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security	BearerAuth
// @Router		/attachment/card/{card_id} [get]
func GetAttachments() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, _, _, handled, err := loadCardForUser(c, ctx, services.ActionCardView)

		if handled {
			return err
		}

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		filter := bson.M{
			"card_id": *card.ID,
			"$or":     services.NotDeleted(),
		}

		kind, err := filters.ParseKind(c.Query("kind"))

		if err != nil {
			return respondQueryError(c, err)
		}

		if kind != "" {
			filter["kind"] = kind
		}

		attachments, total, err := pagination.Fetch[models.Attachment](ctx, database.GetCollection("attachments"), filter, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get attachments")
		}

		return http.Success(c, 200, pagination.NewPage(models.ToAttachmentResponses(attachments), params, total))
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
//
//	@Summary		Anexa um arquivo ao card
//	@Description	O tipo é detectado pelo conteúdo, não pela extensão.
//	@Tags			attachments
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			card_id	path		string	true	"ID do card (ObjectID)"
//	@Param			file	formData	file	true	"Arquivo (png, jpeg, gif, webp, pdf, txt ou zip; até ATTACHMENT_MAX_SIZE_MB)"
//	@Success		201		{object}	dto.AttachmentEnvelope
//	@Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
//	@Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
//	@Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
//	@Failure		409		{object}	dto.ErrorResponse	"Limite de 20 anexos por card"
//	@Failure		413		{object}	dto.ErrorResponse	"Arquivo grande demais"
//	@Security		BearerAuth
//	@Router			/attachment/card/{card_id} [post]
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
//
//	@Summary	Anexa um link ao card
//	@Tags		attachments
//	@Accept		json
//	@Produce	json
//	@Param		card_id	path		string							true	"ID do card (ObjectID)"
//	@Param		body	body		dto.CreateLinkAttachmentRequest	true	"Nome e URL (http ou https)"
//	@Success	201		{object}	dto.AttachmentEnvelope
//	@Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
//	@Failure	401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
//	@Failure	404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
//	@Failure	409		{object}	dto.ErrorResponse	"Limite de 20 anexos por card"
//	@Security	BearerAuth
//	@Router		/attachment/card/{card_id}/link [post]
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
//
//	@Summary		Baixa o arquivo de um anexo
//	@Description	Passa pela API, com checagem de permissão.
//	@Tags			attachments
//	@Produce		application/octet-stream
//	@Param			attachment_id	path		string				true	"ID do anexo (ObjectID)"
//	@Success		200				{file}		file				"Bytes do arquivo"
//	@Failure		400				{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
//	@Failure		401				{object}	dto.ErrorResponse	"Sem token ou token inválido"
//	@Failure		404				{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
//	@Security		BearerAuth
//	@Router			/attachment/{attachment_id}/download [get]
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
//
//	@Summary	Apaga um anexo (soft delete)
//	@Tags		attachments
//	@Accept		json
//	@Produce	json
//	@Param		attachment_id	path		string	true	"ID do anexo (ObjectID)"
//	@Success	200				{object}	dto.StringEnvelope
//	@Failure	400				{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
//	@Failure	401				{object}	dto.ErrorResponse	"Sem token ou token inválido"
//	@Failure	403				{object}	dto.ErrorResponse	"Só quem enviou, ou manager/admin"
//	@Failure	404				{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
//	@Security	BearerAuth
//	@Router		/attachment/{attachment_id} [delete]
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
