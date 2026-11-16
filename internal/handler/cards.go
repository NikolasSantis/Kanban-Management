package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"kanban-management/internal/database"
	cardEvent "kanban-management/internal/events/card"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type createCardRequest struct {
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	ColumnID    string          `json:"column_id"`
	AssigneeIDs []bson.ObjectID `json:"assignee_ids"`
}

// Só estes campos podem ser alterados via PATCH /card/:card_id.
type patchCardRequest struct {
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	AssigneeIDs *[]bson.ObjectID `json:"assignee_ids"`
}

func invalidCardResponse(c *fiber.Ctx, message string) error {
	return http.Error(c, 400, fiber.Map{"error": message}, message)
}

func GetCards() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardID, err := bson.ObjectIDFromHex(c.Params("board_id"))

		if err != nil {
			return invalidCardResponse(c, "Invalid board id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		ctx := context.Background()

		board, err := findActiveBoard(ctx, boardID)

		if err != nil || board.ProjectID == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if _, err := services.AuthorizeProject(ctx, userID, *board.ProjectID, services.ActionCardView); err != nil {
			return respondAuthError(c, err)
		}

		cursor, err := database.GetCollection("cards").Find(ctx, bson.M{
			"board_id": boardID,
			"$or":      services.NotDeleted(),
		}, options.Find().SetSort(bson.D{{Key: "column_id", Value: 1}, {Key: "position", Value: 1}}))

		if err != nil {
			return http.ServerError(c, err, "Fail to get cards")
		}

		defer cursor.Close(ctx)

		cards := []models.Card{}

		if err := cursor.All(ctx, &cards); err != nil {
			return http.ServerError(c, err, "Fail to decode cards")
		}

		return http.Success(c, 200, cards)
	}
}

func CreateCard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardID, err := bson.ObjectIDFromHex(c.Params("board_id"))

		if err != nil {
			return invalidCardResponse(c, "Invalid board id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		ctx := context.Background()

		board, err := findActiveBoard(ctx, boardID)

		if err != nil || board.ProjectID == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if _, err := services.AuthorizeProject(ctx, userID, *board.ProjectID, services.ActionCardCreate); err != nil {
			return respondAuthError(c, err)
		}

		var request createCardRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid body")
		}

		if request.Title == nil {
			return invalidCardResponse(c, "Card title is required")
		}

		title, err := services.NormalizeCardTitle(*request.Title)

		if err != nil {
			return invalidCardResponse(c, "Card title must have 1 to 200 characters")
		}

		if request.Description != nil {
			if err := services.ValidateCardDescription(*request.Description); err != nil {
				return invalidCardResponse(c, "Card description must have at most 5000 characters")
			}
		}

		if !services.ColumnExists(board, request.ColumnID) {
			return invalidCardResponse(c, "Column not found in this board")
		}

		if err := services.ValidateAssignees(ctx, *board.ProjectID, request.AssigneeIDs); err != nil {
			if errors.Is(err, services.ErrInvalidAssignees) {
				return invalidCardResponse(c, "Assignees must be distinct active project members (max 10)")
			}

			return http.ServerError(c, err, "Fail to validate assignees")
		}

		position, err := services.NextPosition(ctx, boardID, request.ColumnID)

		if err != nil {
			return http.ServerError(c, err, "Fail to calculate card position")
		}

		now := time.Now()

		assignees := request.AssigneeIDs

		if assignees == nil {
			assignees = []bson.ObjectID{}
		}

		card := models.Card{
			ProjectID:   board.ProjectID,
			BoardID:     &boardID,
			ColumnID:    &request.ColumnID,
			Title:       &title,
			Description: request.Description,
			Position:    position,
			AssigneeIDs: assignees,
			CreatedBy:   &userID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		result, err := database.GetCollection("cards").InsertOne(ctx, card)

		if err != nil {
			return http.ServerError(c, err, "Card not created")
		}

		insertedID, ok := result.InsertedID.(bson.ObjectID)

		if !ok {
			return http.ServerError(c, nil, "Card not created")
		}

		card.ID = &insertedID

		slog.Info("card.created", "card_id", insertedID.Hex(), "board_id", boardID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 201, card)
	}
}

// loadCardForUser carrega o card ativo e confere a permissão no projeto dele.
// Em caso de erro, a resposta HTTP já foi escrita (handled=true).
func loadCardForUser(c *fiber.Ctx, ctx context.Context, action services.Action) (card *models.Card, userID bson.ObjectID, role string, handled bool, err error) {
	cardID, err := bson.ObjectIDFromHex(c.Params("card_id"))

	if err != nil {
		return nil, userID, "", true, invalidCardResponse(c, "Invalid card id")
	}

	userID, err = middleware.CurrentUserID(c)

	if err != nil {
		return nil, userID, "", true, http.Error(c, 401, fiber.Map{
			"error": "Fail to get user auth",
		}, "Fail to get user auth")
	}

	card, err = services.FindActiveCard(ctx, cardID)

	if err != nil || card.ProjectID == nil || card.BoardID == nil {
		return nil, userID, "", true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	role, err = services.AuthorizeProject(ctx, userID, *card.ProjectID, action)

	if err != nil {
		return nil, userID, role, true, respondAuthError(c, err)
	}

	return card, userID, role, false, nil
}

func GetCard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		card, _, _, handled, err := loadCardForUser(c, context.Background(), services.ActionCardView)

		if handled {
			return err
		}

		return http.Success(c, 200, card)
	}
}

func PatchCard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, userID, _, handled, err := loadCardForUser(c, ctx, services.ActionCardUpdate)

		if handled {
			return err
		}

		var request patchCardRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid JSON")
		}

		updates := bson.M{}

		if request.Title != nil {
			title, err := services.NormalizeCardTitle(*request.Title)

			if err != nil {
				return invalidCardResponse(c, "Card title must have 1 to 200 characters")
			}

			updates["title"] = title
		}

		if request.Description != nil {
			if err := services.ValidateCardDescription(*request.Description); err != nil {
				return invalidCardResponse(c, "Card description must have at most 5000 characters")
			}

			updates["description"] = *request.Description
		}

		if request.AssigneeIDs != nil {
			assignees := *request.AssigneeIDs

			if err := services.ValidateAssignees(ctx, *card.ProjectID, assignees); err != nil {
				if errors.Is(err, services.ErrInvalidAssignees) {
					return invalidCardResponse(c, "Assignees must be distinct active project members (max 10)")
				}

				return http.ServerError(c, err, "Fail to validate assignees")
			}

			if assignees == nil {
				assignees = []bson.ObjectID{}
			}

			updates["assignee_ids"] = assignees
		}

		if len(updates) == 0 {
			return invalidCardResponse(c, "No valid field to update")
		}

		updates["updated_at"] = time.Now()

		_, err = database.GetCollection("cards").UpdateByID(ctx, *card.ID, bson.M{"$set": updates})

		if err != nil {
			return http.ServerError(c, err, "Error to update card")
		}

		slog.Info("card.updated", "card_id", card.ID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "Updated")
	}
}

func DeleteCard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		// Qualquer membro enxerga o card; quem pode apagar é decidido abaixo.
		card, userID, role, handled, err := loadCardForUser(c, ctx, services.ActionCardView)

		if handled {
			return err
		}

		isCreator := card.CreatedBy != nil && *card.CreatedBy == userID

		if !services.Can(role, services.ActionCardDelete) && !isCreator {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		now := time.Now()

		result, err := database.GetCollection("cards").UpdateByID(ctx, *card.ID, bson.M{
			"$set": bson.M{
				"deleted_at": now,
				"updated_at": now,
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete card")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No card modified")
		}

		err = services.Dispatcher.Dispatch(
			cardEvent.CardDeletedEvent{
				CardID: *card.ID,
				Ctx:    ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch card deleted event")
		}

		slog.Info("card.deleted", "card_id", card.ID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
