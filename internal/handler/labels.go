package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"kanban-management/internal/database"
	labelEvent "kanban-management/internal/events/label"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type labelRequest struct {
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

const duplicateLabelMessage = "Label name already exists on this board"

func GetLabels() fiber.Handler {
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

		if _, err := services.AuthorizeProject(ctx, userID, *board.ProjectID, services.ActionBoardView); err != nil {
			return respondAuthError(c, err)
		}

		cursor, err := database.GetCollection("labels").Find(ctx, bson.M{
			"board_id": boardID,
			"$or":      services.NotDeleted(),
		}, options.Find().SetSort(bson.D{{Key: "name_normalized", Value: 1}}))

		if err != nil {
			return http.ServerError(c, err, "Fail to get labels")
		}

		defer cursor.Close(ctx)

		labels := []models.Label{}

		if err := cursor.All(ctx, &labels); err != nil {
			return http.ServerError(c, err, "Fail to decode labels")
		}

		return http.Success(c, 200, labels)
	}
}

func CreateLabel() fiber.Handler {
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

		if _, err := services.AuthorizeProject(ctx, userID, *board.ProjectID, services.ActionLabelManage); err != nil {
			return respondAuthError(c, err)
		}

		var request labelRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid body")
		}

		if request.Name == nil {
			return invalidCardResponse(c, "Label name is required")
		}

		name, err := services.CleanLabelName(*request.Name)

		if err != nil {
			return invalidCardResponse(c, "Label name must have 1 to 30 characters")
		}

		count, err := services.ActiveLabelCount(ctx, boardID)

		if err != nil {
			return http.ServerError(c, err, "Fail to count labels")
		}

		if count >= services.MaxLabelsPerBoard {
			return invalidCardResponse(c, "A board can have at most 50 labels")
		}

		var color string

		if request.Color != nil {
			if !services.ValidateColor(*request.Color) {
				return invalidCardResponse(c, "Invalid color. Use #RRGGBB")
			}

			color = *request.Color
		} else {
			color = services.PaletteColor(int(count))
		}

		now := time.Now()

		label := models.Label{
			ProjectID:      board.ProjectID,
			BoardID:        &boardID,
			Name:           &name,
			NameNormalized: services.NormalizeLabelName(name),
			Color:          &color,
			Active:         true,
			CreatedBy:      &userID,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		result, err := database.GetCollection("labels").InsertOne(ctx, label)

		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return http.Error(c, 409, fiber.Map{
					"error": duplicateLabelMessage,
				}, duplicateLabelMessage)
			}

			return http.ServerError(c, err, "Label not created")
		}

		insertedID, ok := result.InsertedID.(bson.ObjectID)

		if !ok {
			return http.ServerError(c, nil, "Label not created")
		}

		label.ID = &insertedID

		slog.Info("label.created", "label_id", insertedID.Hex(), "board_id", boardID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 201, label)
	}
}

// loadLabelForManage carrega a label ativa e exige label:manage no projeto dela.
func loadLabelForManage(c *fiber.Ctx, ctx context.Context) (label *models.Label, userID bson.ObjectID, handled bool, err error) {
	labelID, err := bson.ObjectIDFromHex(c.Params("label_id"))

	if err != nil {
		return nil, userID, true, invalidCardResponse(c, "Invalid label id")
	}

	userID, err = middleware.CurrentUserID(c)

	if err != nil {
		return nil, userID, true, http.Error(c, 401, fiber.Map{
			"error": "Fail to get user auth",
		}, "Fail to get user auth")
	}

	label, err = services.FindActiveLabel(ctx, labelID)

	if err != nil || label.ProjectID == nil || label.BoardID == nil {
		return nil, userID, true, http.Error(c, 404, fiber.Map{
			"error": "Not found",
		}, "Not found")
	}

	if _, err := services.AuthorizeProject(ctx, userID, *label.ProjectID, services.ActionLabelManage); err != nil {
		return nil, userID, true, respondAuthError(c, err)
	}

	return label, userID, false, nil
}

func PatchLabel() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		label, userID, handled, err := loadLabelForManage(c, ctx)

		if handled {
			return err
		}

		var request labelRequest

		if err := json.Unmarshal(c.Body(), &request); err != nil {
			return invalidCardResponse(c, "Invalid JSON")
		}

		updates := bson.M{}

		if request.Name != nil {
			name, err := services.CleanLabelName(*request.Name)

			if err != nil {
				return invalidCardResponse(c, "Label name must have 1 to 30 characters")
			}

			updates["name"] = name
			updates["name_normalized"] = services.NormalizeLabelName(name)
		}

		if request.Color != nil {
			if !services.ValidateColor(*request.Color) {
				return invalidCardResponse(c, "Invalid color. Use #RRGGBB")
			}

			updates["color"] = *request.Color
		}

		if len(updates) == 0 {
			return invalidCardResponse(c, "No valid field to update")
		}

		updates["updated_at"] = time.Now()

		_, err = database.GetCollection("labels").UpdateByID(ctx, *label.ID, bson.M{"$set": updates})

		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return http.Error(c, 409, fiber.Map{
					"error": duplicateLabelMessage,
				}, duplicateLabelMessage)
			}

			return http.ServerError(c, err, "Error to update label")
		}

		slog.Info("label.updated", "label_id", label.ID.Hex(), "user_id", userID.Hex())

		updated, err := services.FindActiveLabel(ctx, *label.ID)

		if err != nil {
			return http.ServerError(c, err, "Error to load label")
		}

		return http.Success(c, 200, updated)
	}
}

func DeleteLabel() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx := context.Background()

		label, userID, handled, err := loadLabelForManage(c, ctx)

		if handled {
			return err
		}

		now := time.Now()

		// deleted_at E active=false: o índice único parcial usa "active", e é isso
		// que libera o nome para ser criado de novo.
		result, err := database.GetCollection("labels").UpdateByID(ctx, *label.ID, bson.M{
			"$set": bson.M{
				"deleted_at": now,
				"updated_at": now,
				"active":     false,
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete label")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No label modified")
		}

		err = services.Dispatcher.Dispatch(
			labelEvent.LabelDeletedEvent{
				LabelID: *label.ID,
				BoardID: *label.BoardID,
				Ctx:     ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch label deleted event")
		}

		slog.Info("label.deleted", "label_id", label.ID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
