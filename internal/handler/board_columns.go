package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const maxColumnNameLength = 100

type columnRequest struct {
	ID       *string `json:"id"`
	Name     *string `json:"name"`
	Color    *string `json:"color"`
	Icon     *string `json:"icon"`
	Position *int32  `json:"position"`
}

// loadBoardForColumns carrega o board ativo e exige board:manage no projeto.
// Se handled=true, a resposta HTTP já foi escrita.
func loadBoardForColumns(c *fiber.Ctx, ctx context.Context) (board *models.Board, userID bson.ObjectID, handled bool, err error) {
	boardID, err := bson.ObjectIDFromHex(c.Params("board_id"))

	if err != nil {
		return nil, userID, true, http.Error(c, 400, fiber.Map{
			"error": "Invalid board id",
		}, "Invalid board id")
	}

	userID, err = middleware.CurrentUserID(c)

	if err != nil {
		return nil, userID, true, http.Error(c, 401, fiber.Map{
			"error": "Fail to get user auth",
		}, "Fail to get user auth")
	}

	board, err = findActiveBoard(ctx, boardID)

	if err != nil || board.ProjectID == nil || board.ID == nil {
		return nil, userID, true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	if _, err := services.AuthorizeProject(ctx, userID, *board.ProjectID, services.ActionBoardManage); err != nil {
		return nil, userID, true, respondAuthError(c, err)
	}

	return board, userID, false, nil
}

func validColumnName(name string) (string, bool) {
	name = strings.TrimSpace(name)

	length := utf8.RuneCountInString(name)

	return name, length >= 1 && length <= maxColumnNameLength
}

func AddColumn() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		board, userID, handled, err := loadBoardForColumns(c, ctx)

		if handled {
			return err
		}

		var request columnRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid body")
		}

		if request.Name == nil {
			return invalidCardResponse(c, "Column name is required")
		}

		name, ok := validColumnName(*request.Name)

		if !ok {
			return invalidCardResponse(c, "Column name must have 1 to 100 characters")
		}

		columnID := bson.NewObjectID().Hex()

		if request.ID != nil {
			columnID = strings.TrimSpace(*request.ID)

			if columnID == "" {
				return invalidCardResponse(c, "Invalid column id")
			}
		}

		// O id não pode repetir, nem o de uma coluna já deletada.
		maxPosition := int32(0)

		for _, column := range board.Columns {
			if column.ID != nil && *column.ID == columnID {
				return http.Error(c, 409, fiber.Map{
					"error": "Column id already exists",
				}, "Column id already exists")
			}

			if column.DeletedAt == nil && column.Position != nil && *column.Position > maxPosition {
				maxPosition = *column.Position
			}
		}

		position := maxPosition + 1

		if request.Position != nil {
			position = *request.Position
		}

		now := time.Now()

		column := models.Column{
			ID:        &columnID,
			Name:      &name,
			Position:  &position,
			Color:     request.Color,
			Icon:      request.Icon,
			CreatedAt: now,
			UpdatedAt: now,
		}

		_, err = database.GetCollection("boards").UpdateByID(ctx, *board.ID, bson.M{
			"$push": bson.M{"columns": column},
			"$set":  bson.M{"updated_at": now},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to add column")
		}

		slog.Info("column.created", "board_id", board.ID.Hex(), "column_id", columnID, "user_id", userID.Hex())

		return http.Success(c, 201, column)
	}
}

func PatchColumn() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		board, userID, handled, err := loadBoardForColumns(c, ctx)

		if handled {
			return err
		}

		columnID := c.Params("column_id")

		if !services.ColumnExists(board, columnID) {
			return http.Error(c, 404, fiber.Map{
				"error": "Column not found",
			}, "Column not found")
		}

		var request columnRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid JSON")
		}

		now := time.Now()
		updates := bson.M{}

		if request.Name != nil {
			name, ok := validColumnName(*request.Name)

			if !ok {
				return invalidCardResponse(c, "Column name must have 1 to 100 characters")
			}

			updates["columns.$[col].name"] = name
		}

		if request.Color != nil {
			updates["columns.$[col].color"] = *request.Color
		}

		if request.Icon != nil {
			updates["columns.$[col].icon"] = *request.Icon
		}

		if request.Position != nil {
			updates["columns.$[col].position"] = *request.Position
		}

		if len(updates) == 0 {
			return invalidCardResponse(c, "No valid field to update")
		}

		updates["columns.$[col].updated_at"] = now
		updates["updated_at"] = now

		_, err = database.GetCollection("boards").UpdateByID(ctx, *board.ID,
			bson.M{"$set": updates},
			options.UpdateOne().SetArrayFilters([]any{bson.M{"col.id": columnID}}),
		)

		if err != nil {
			return http.ServerError(c, err, "Error to update column")
		}

		slog.Info("column.updated", "board_id", board.ID.Hex(), "column_id", columnID, "user_id", userID.Hex())

		return http.Success(c, 200, "Updated")
	}
}

func DeleteColumn() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		board, userID, handled, err := loadBoardForColumns(c, ctx)

		if handled {
			return err
		}

		columnID := c.Params("column_id")

		if !services.ColumnExists(board, columnID) {
			return http.Error(c, 404, fiber.Map{
				"error": "Column not found",
			}, "Column not found")
		}

		count, err := services.ActiveCardCount(ctx, *board.ID, columnID)

		if err != nil {
			return http.ServerError(c, err, "Fail to check column cards")
		}

		if count > 0 {
			return http.Error(c, 409, fiber.Map{
				"error": "Column has active cards",
			}, "Column has active cards")
		}

		now := time.Now()

		_, err = database.GetCollection("boards").UpdateByID(ctx, *board.ID,
			bson.M{"$set": bson.M{
				"columns.$[col].deleted_at": now,
				"columns.$[col].updated_at": now,
				"updated_at":                now,
			}},
			options.UpdateOne().SetArrayFilters([]any{bson.M{"col.id": columnID}}),
		)

		if err != nil {
			return http.ServerError(c, err, "Error to delete column")
		}

		slog.Info("column.deleted", "board_id", board.ID.Hex(), "column_id", columnID, "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
