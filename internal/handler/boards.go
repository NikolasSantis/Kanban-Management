package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	boardEvent "kanban-management/internal/events/board"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/pagination"
	"kanban-management/internal/services"
	"log/slog"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Campos que não podem ser alterados via PATCH
var boardProtectedFields = []string{
	"_id",
	"id",
	"project_id",
	"created_at",
	"updated_at",
	"deleted_at",
}

// findActiveBoard busca um board que não foi deletado (soft delete)
func findActiveBoard(ctx context.Context, boardID bson.ObjectID) (*models.Board, error) {
	var board models.Board

	err := database.GetCollection("boards").FindOne(ctx, bson.M{
		"_id": boardID,
		"$or": services.NotDeleted(),
	}).Decode(&board)

	if err != nil {
		return nil, err
	}

	return &board, nil
}

func GetBoards() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Param",
			}, "Missing Param")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project id",
			}, "Invalid project id")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionBoardView); err != nil {
			return respondAuthError(c, err)
		}

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		boards, total, err := pagination.Fetch[models.Board](context.Background(), database.GetCollection("boards"), bson.M{
			"project_id": projectID,
			"$or":        services.NotDeleted(),
		}, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get project boards")
		}

		return http.Success(c, 200, pagination.NewPage(boards, params, total))
	}
}

func GetBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardIDHex := c.Params("board_id")

		if boardIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing board id",
			}, "Missing board id")
		}

		boardID, err := bson.ObjectIDFromHex(boardIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid board id",
			}, "Invalid board id")
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

		if _, err := services.AuthorizeProject(context.Background(), userID, *board.ProjectID, services.ActionBoardView); err != nil {
			return respondAuthError(c, err)
		}

		return http.Success(c, 200, board)
	}
}

func CreateBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Param",
			}, "Missing Param")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project id",
			}, "Invalid project id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionBoardManage); err != nil {
			return respondAuthError(c, err)
		}

		var board models.Board

		if err := c.BodyParser(&board); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		if board.Name == nil || *board.Name == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Board name is required",
			}, "Board name is required")
		}

		now := time.Now()

		board.ID = nil
		board.ProjectID = &projectID
		board.DeletedAt = nil
		board.CreatedAt, board.UpdatedAt = now, now

		for i := range board.Columns {
			board.Columns[i].CreatedAt = now
			board.Columns[i].UpdatedAt = now
		}

		for i := range board.Members {
			board.Members[i].CreatedAt = now
			board.Members[i].UpdatedAt = now
		}

		ctx := context.Background()
		collection := database.GetCollection("boards")

		result, err := collection.InsertOne(ctx, board)

		if err != nil {
			return http.ServerError(c, err, "Board not created")
		}

		if !result.Acknowledged {
			return http.ServerError(c, nil, "Error to create board")
		}

		if insertedID, ok := result.InsertedID.(bson.ObjectID); ok {
			board.ID = &insertedID
		}

		slog.Info("board.created", "board_id", board.ID, "project_id", projectID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 201, board)
	}
}

func PatchBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardIDHex := c.Params("board_id")

		if boardIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing board id",
			}, "Missing board id")
		}

		boardID, err := bson.ObjectIDFromHex(boardIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid board id",
			}, "Invalid board id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		var body map[string]any

		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid JSON",
			}, "Invalid JSON")
		}

		updates := make(map[string]any)

		for key, value := range body {
			if slices.Contains(boardProtectedFields, key) {
				continue
			}

			updates[key] = value
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid field to update",
			}, "No valid field to update")
		}

		ctx := context.Background()

		board, err := findActiveBoard(ctx, boardID)

		if err != nil || board.ProjectID == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, *board.ProjectID, services.ActionBoardManage); err != nil {
			return respondAuthError(c, err)
		}

		updates["updated_at"] = time.Now()

		collection := database.GetCollection("boards")

		result, err := collection.UpdateByID(ctx, boardID, bson.M{
			"$set": updates,
		})

		if err != nil {
			return http.ServerError(c, err, "Error to update")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "Not data updated")
		}

		return http.Success(c, 200, "Updated")
	}
}

func DeleteBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardIDHex := c.Params("board_id")

		if boardIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing board id",
			}, "Missing board id")
		}

		boardID, err := bson.ObjectIDFromHex(boardIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid board id",
			}, "Invalid board id")
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

		if _, err := services.AuthorizeProject(context.Background(), userID, *board.ProjectID, services.ActionBoardManage); err != nil {
			return respondAuthError(c, err)
		}

		now := time.Now()

		collection := database.GetCollection("boards")

		result, err := collection.UpdateByID(ctx, boardID, bson.M{
			"$set": bson.M{
				"deleted_at": now,
				"updated_at": now,
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete board")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No board modified")
		}

		err = services.Dispatcher.Dispatch(
			boardEvent.BoardDeletedEvent{
				BoardID: boardID,
				Ctx:     ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch board deleted event")
		}

		slog.Info("board.deleted", "board_id", boardID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
