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
	Priority    *string         `json:"priority"`
}

// Só estes campos podem ser alterados via PATCH /card/:card_id.
type patchCardRequest struct {
	Title       *string          `json:"title"`
	Description *string          `json:"description"`
	AssigneeIDs *[]bson.ObjectID `json:"assignee_ids"`
	Priority    *string          `json:"priority"`
}

type moveCardRequest struct {
	ColumnID     string         `json:"column_id"`
	BeforeCardID *bson.ObjectID `json:"before_card_id"`
	AfterCardID  *bson.ObjectID `json:"after_card_id"`
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

		priority := models.PriorityDefault

		if request.Priority != nil {
			priority, err = models.ParsePriority(*request.Priority)

			if err != nil {
				return invalidCardResponse(c, models.InvalidPriorityMessage)
			}
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
			ProjectID:     board.ProjectID,
			BoardID:       &boardID,
			ColumnID:      &request.ColumnID,
			Title:         &title,
			Description:   request.Description,
			Position:      position,
			Priority:      priority,
			PriorityLevel: priority.Level(),
			AssigneeIDs:   assignees,
			CreatedBy:     &userID,
			CreatedAt:     now,
			UpdatedAt:     now,
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

		if request.Priority != nil {
			priority, err := models.ParsePriority(*request.Priority)

			if err != nil {
				return invalidCardResponse(c, models.InvalidPriorityMessage)
			}

			updates["priority"] = priority
			updates["priority_level"] = priority.Level()
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

func MoveCard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		card, userID, _, handled, err := loadCardForUser(c, ctx, services.ActionCardUpdate)

		if handled {
			return err
		}

		var request moveCardRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid body")
		}

		board, err := findActiveBoard(ctx, *card.BoardID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if !services.ColumnExists(board, request.ColumnID) {
			return invalidCardResponse(c, "Column not found in this board")
		}

		before, err := loadNeighbor(ctx, request.BeforeCardID, card, request.ColumnID)

		if err != nil {
			return invalidCardResponse(c, "before_card_id must be another card in the destination column of this board")
		}

		after, err := loadNeighbor(ctx, request.AfterCardID, card, request.ColumnID)

		if err != nil {
			return invalidCardResponse(c, "after_card_id must be another card in the destination column of this board")
		}

		if before != nil && after != nil && before.Position >= after.Position {
			return invalidCardResponse(c, "before_card_id must be above after_card_id")
		}

		position, err := resolveMovePosition(ctx, card, request.ColumnID, before, after)

		if err != nil {
			return http.ServerError(c, err, "Fail to calculate card position")
		}

		now := time.Now()

		_, err = database.GetCollection("cards").UpdateByID(ctx, *card.ID, bson.M{
			"$set": bson.M{
				"column_id":  request.ColumnID,
				"position":   position,
				"updated_at": now,
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to move card")
		}

		slog.Info("card.moved", "card_id", card.ID.Hex(), "column_id", request.ColumnID, "user_id", userID.Hex())

		card.ColumnID = &request.ColumnID
		card.Position = position
		card.UpdatedAt = now

		return http.Success(c, 200, card)
	}
}

// loadNeighbor carrega o card de referência do movimento e confere que ele é
// outro card, ativo, do mesmo board e da coluna de destino.
func loadNeighbor(ctx context.Context, id *bson.ObjectID, moving *models.Card, columnID string) (*models.Card, error) {
	if id == nil {
		return nil, nil
	}

	if *id == *moving.ID {
		return nil, errors.New("neighbor is the moved card")
	}

	neighbor, err := services.FindActiveCard(ctx, *id)

	if err != nil {
		return nil, err
	}

	if neighbor.BoardID == nil || *neighbor.BoardID != *moving.BoardID || neighbor.ColumnID == nil || *neighbor.ColumnID != columnID {
		return nil, errors.New("neighbor is not in the destination column")
	}

	return neighbor, nil
}

// resolveMovePosition calcula a position final. Com só um vizinho informado, o
// outro lado é descoberto na coluna. Se o espaço entre os dois acabou, a coluna
// é renumerada e o cálculo é refeito.
func resolveMovePosition(ctx context.Context, moving *models.Card, columnID string, before, after *models.Card) (float64, error) {
	boardID := *moving.BoardID

	for attempt := 0; attempt < 2; attempt++ {
		var beforePos, afterPos *float64

		switch {
		case before != nil && after != nil:
			beforePos, afterPos = &before.Position, &after.Position
		case before != nil:
			next, err := services.NeighborPosition(ctx, boardID, columnID, before.Position, *moving.ID, true)

			if err != nil {
				return 0, err
			}

			beforePos, afterPos = &before.Position, next
		case after != nil:
			prev, err := services.NeighborPosition(ctx, boardID, columnID, after.Position, *moving.ID, false)

			if err != nil {
				return 0, err
			}

			beforePos, afterPos = prev, &after.Position
		default:
			last, err := services.LastPositionExcluding(ctx, boardID, columnID, *moving.ID)

			if err != nil {
				return 0, err
			}

			beforePos = last
		}

		if beforePos != nil && afterPos != nil && *afterPos-*beforePos < services.MinPositionGap {
			if err := services.RenumberColumn(ctx, boardID, columnID); err != nil {
				return 0, err
			}

			// Recarrega as positions já renumeradas e tenta de novo.
			if before != nil {
				fresh, err := services.FindActiveCard(ctx, *before.ID)

				if err != nil {
					return 0, err
				}

				before = fresh
			}

			if after != nil {
				fresh, err := services.FindActiveCard(ctx, *after.ID)

				if err != nil {
					return 0, err
				}

				after = fresh
			}

			continue
		}

		return services.PositionBetween(beforePos, afterPos), nil
	}

	return 0, errors.New("could not find room between cards after renumbering")
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
