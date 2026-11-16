package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/pagination"
	"log/slog"
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type commentRequest struct {
	Content *string `json:"content"`
}

func invalidCommentResponse(c *fiber.Ctx) error {
	return invalidCardResponse(c, "Comment content must have 1 to 5000 characters")
}

// commentResponse monta a resposta de um comentário (com o autor).
func commentResponse(ctx context.Context, comment models.Comment) (models.CommentResponse, error) {
	ids := []bson.ObjectID{}

	if comment.AuthorID != nil {
		ids = append(ids, *comment.AuthorID)
	}

	authors, err := services.LoadAuthors(ctx, ids)

	if err != nil {
		return models.CommentResponse{}, err
	}

	return services.ToCommentResponses([]models.Comment{comment}, authors)[0], nil
}

func GetComments() fiber.Handler {
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

		comments, total, err := pagination.Fetch[models.Comment](ctx, database.GetCollection("comments"), bson.M{
			"card_id": *card.ID,
			"$or":     services.NotDeleted(),
		}, bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get comments")
		}

		// UMA query para todos os autores da página.
		authorIDs := make([]bson.ObjectID, 0, len(comments))
		seen := map[bson.ObjectID]bool{}

		for _, comment := range comments {
			if comment.AuthorID != nil && !seen[*comment.AuthorID] {
				seen[*comment.AuthorID] = true
				authorIDs = append(authorIDs, *comment.AuthorID)
			}
		}

		authors, err := services.LoadAuthors(ctx, authorIDs)

		if err != nil {
			return http.ServerError(c, err, "Fail to load comment authors")
		}

		return http.Success(c, 200, pagination.NewPage(services.ToCommentResponses(comments, authors), params, total))
	}
}

func CreateComment() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, userID, _, handled, err := loadCardForUser(c, ctx, services.ActionCommentCreate)

		if handled {
			return err
		}

		var request commentRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid body")
		}

		if request.Content == nil {
			return invalidCommentResponse(c)
		}

		content, err := services.ValidateCommentContent(*request.Content)

		if err != nil {
			return invalidCommentResponse(c)
		}

		now := time.Now()

		comment := models.Comment{
			ProjectID: card.ProjectID,
			BoardID:   card.BoardID,
			CardID:    card.ID,
			AuthorID:  &userID,
			Content:   &content,
			CreatedAt: now,
			UpdatedAt: now,
		}

		result, err := database.GetCollection("comments").InsertOne(ctx, comment)

		if err != nil {
			return http.ServerError(c, err, "Comment not created")
		}

		insertedID, ok := result.InsertedID.(bson.ObjectID)

		if !ok {
			return http.ServerError(c, nil, "Comment not created")
		}

		comment.ID = &insertedID

		slog.Info("comment.created", "comment_id", insertedID.Hex(), "card_id", card.ID.Hex(), "user_id", userID.Hex())

		response, err := commentResponse(ctx, comment)

		if err != nil {
			return http.ServerError(c, err, "Fail to load comment author")
		}

		return http.Success(c, 201, response)
	}
}

// loadCommentForUser carrega o comentário ativo e confere que o usuário ainda é
// membro do projeto (devolve a role). handled=true: a resposta já foi escrita.
func loadCommentForUser(c *fiber.Ctx, ctx context.Context) (comment *models.Comment, userID bson.ObjectID, role string, handled bool, err error) {
	commentID, err := bson.ObjectIDFromHex(c.Params("comment_id"))

	if err != nil {
		return nil, userID, "", true, invalidCardResponse(c, "Invalid comment id")
	}

	userID, err = middleware.CurrentUserID(c)

	if err != nil {
		return nil, userID, "", true, http.Error(c, 401, fiber.Map{
			"error": "Fail to get user auth",
		}, "Fail to get user auth")
	}

	comment, err = services.FindActiveComment(ctx, commentID)

	if err != nil || comment.ProjectID == nil || comment.CardID == nil {
		return nil, userID, "", true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	// O card do comentário precisa continuar ativo.
	if _, err := services.FindActiveCard(ctx, *comment.CardID); err != nil {
		return nil, userID, "", true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	role, err = services.AuthorizeProject(ctx, userID, *comment.ProjectID, services.ActionCardView)

	if err != nil {
		return nil, userID, role, true, respondAuthError(c, err)
	}

	return comment, userID, role, false, nil
}

func isCommentAuthor(comment *models.Comment, userID bson.ObjectID) bool {
	return comment.AuthorID != nil && *comment.AuthorID == userID
}

// PatchComment: só o autor edita (nem admin edita texto alheio).
func PatchComment() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		comment, userID, _, handled, err := loadCommentForUser(c, ctx)

		if handled {
			return err
		}

		if !isCommentAuthor(comment, userID) {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		var request commentRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid JSON")
		}

		if request.Content == nil {
			return invalidCommentResponse(c)
		}

		content, err := services.ValidateCommentContent(*request.Content)

		if err != nil {
			return invalidCommentResponse(c)
		}

		now := time.Now()

		_, err = database.GetCollection("comments").UpdateByID(ctx, *comment.ID, bson.M{
			"$set": bson.M{"content": content, "edited_at": now, "updated_at": now},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to update comment")
		}

		slog.Info("comment.updated", "comment_id", comment.ID.Hex(), "user_id", userID.Hex())

		updated, err := services.FindActiveComment(ctx, *comment.ID)

		if err != nil {
			return http.ServerError(c, err, "Error to load comment")
		}

		response, err := commentResponse(ctx, *updated)

		if err != nil {
			return http.ServerError(c, err, "Fail to load comment author")
		}

		return http.Success(c, 200, response)
	}
}

// DeleteComment: o autor ou quem tem comment:moderate (manager/admin).
func DeleteComment() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		comment, userID, role, handled, err := loadCommentForUser(c, ctx)

		if handled {
			return err
		}

		if !isCommentAuthor(comment, userID) && !services.Can(role, services.ActionCommentModerate) {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		now := time.Now()

		result, err := database.GetCollection("comments").UpdateByID(ctx, *comment.ID, bson.M{
			"$set": bson.M{"deleted_at": now, "deleted_by": userID, "updated_at": now},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete comment")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No comment modified")
		}

		slog.Info("comment.deleted", "comment_id", comment.ID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
