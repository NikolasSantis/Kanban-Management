package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	"kanban-management/internal/events/workspace"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/pagination"
	"kanban-management/internal/services"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Campos que o PATCH de workspace aceita (allowlist).
var workspacePatchableFields = map[string]bool{
	"name":        true,
	"description": true,
}

func GetWorkspaces() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.ServerError(c, err, "Fail to user auth")
		}

		collection := database.GetCollection("workspaces")
		ctx := context.Background()

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		workspaces, total, err := pagination.Fetch[models.Workspace](ctx, collection, bson.M{
			"owner_id": userID,
			"$or":      services.NotDeleted(),
		}, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Error to get workspaces")
		}

		return http.Success(c, 200, pagination.NewPage(workspaces, params, total))
	}
}

func CreateWorkspace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var workspace models.Workspace

		if err := c.BodyParser(&workspace); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to the user auth")
		}

		workspace.OwnderID = userID

		now := time.Now()

		workspace.CreatedAt = now
		workspace.UpdatedAt = now

		collection := database.GetCollection("workspaces")
		ctx := context.Background()

		result, err := collection.InsertOne(ctx, workspace)

		if err != nil {
			return http.ServerError(c, err, "Fail to create workspace")
		}

		insertedID, ok := result.InsertedID.(bson.ObjectID)

		if !ok {
			return http.ServerError(c, nil, "Fail to create workspace")
		}

		workspace.ID = insertedID

		ownerAsMember := services.AddOwnerProjectAsOneMember(userID, insertedID)

		if !ownerAsMember {
			return http.ServerError(c, nil, "Fail to save owner as one workspace member")
		}

		slog.Info("workspace.created", "workspace_id", insertedID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, workspace)
	}
}

func PatchWorkspace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIDHex := c.Params("id")

		if workspaceIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing param",
			}, "Missing param")
		}

		workspaceID, err := bson.ObjectIDFromHex(workspaceIDHex)

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Error on workspace ID",
			}, "Error on workspace ID")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to user auth")
		}

		ctx := context.Background()

		if _, err := services.AuthorizeWorkspace(ctx, userID, workspaceID, services.ActionWorkspaceUpdate); err != nil {
			return respondAuthError(c, err)
		}

		var body map[string]any

		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid JSON",
			}, "Invalid JSON")
		}

		updates := make(map[string]any)

		for key, value := range body {
			if workspacePatchableFields[key] {
				updates[key] = value
			}
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid fields to update",
			}, "No valid fields to update")
		}

		updates["updated_at"] = time.Now()

		collection := database.GetCollection("workspaces")

		updateResult, err := collection.UpdateByID(ctx, workspaceID, bson.M{
			"$set": updates,
		})

		if err != nil {
			return http.ServerError(c, err, "Error to update register")
		}

		if updateResult.ModifiedCount != 1 {
			return http.ServerError(c, nil, "No modified register")
		}

		return http.Success(c, 200, fiber.Map{"message": "Updated"})
	}
}

func DeleteWorkspace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIDHex := c.Params("id")

		if workspaceIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing param",
			}, "Missing param")
		}

		workspaceID, err := bson.ObjectIDFromHex(workspaceIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invlalid workspace ID",
			}, "Invalid workspace ID")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.ServerError(c, err, "Fail to user auth")
		}

		ctx := context.Background()

		if _, err := services.AuthorizeWorkspace(ctx, userID, workspaceID, services.ActionWorkspaceDelete); err != nil {
			return respondAuthError(c, err)
		}

		collection := database.GetCollection("workspaces")

		result := collection.FindOneAndUpdate(ctx, bson.M{
			"_id": workspaceID,
			"$or": services.NotDeleted(),
		}, bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		})

		if result.Err() != nil {
			return http.ServerError(c, result.Err(), "Delete operation error")
		}

		err = services.Dispatcher.Dispatch(
			workspace.WorkspaceDeletedEvent{
				WorkspaceID: workspaceID,
				Ctx:         ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch event")
		}

		slog.Info("workspace.deleted", "workspace_id", workspaceID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "Workspace deleted")
	}
}
