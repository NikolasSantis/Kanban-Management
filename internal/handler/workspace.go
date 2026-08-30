package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	"kanban-management/internal/events/workspace"
	"kanban-management/internal/http"
	"kanban-management/internal/models"
	"kanban-management/internal/services"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetWorkspaces() fiber.Handler {
	return func(c *fiber.Ctx) error {
		objectID := c.Locals("user_id").(string)

		userID, err := bson.ObjectIDFromHex(objectID)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to user auth")
		}

		collection := database.GetCollection("workspaces")
		ctx := context.Background()

		cursor, err := collection.Find(ctx, bson.M{
			"owner_id": userID,
		})

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspaces not found",
			}, "Workspaces not found")
		}

		defer cursor.Close(ctx)

		var workspaces []models.Workspace

		if err = cursor.All(ctx, &workspaces); err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Erro to verify workspaces",
			}, "Error to verify workspaces")
		}

		return http.Success(c, 200, workspaces)
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

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

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
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to create workspace",
			}, "Fail to create workspace")
		}

		ownerAsMember := services.AddOwnerProjectAsOneMember(userID, result.InsertedID.(bson.ObjectID))

		if !ownerAsMember {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to save owner as one workspace member",
			}, "Fail to save owner as one workspace member")
		}

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

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		userRole, err := services.GetWorkspaceUserRole(userID, workspaceID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		var workspace models.Workspace
		var body map[string]any

		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid JSON",
			}, "Invalid JSON")
		}

		updates := make(map[string]any)

		for key, value := range body {
			updates[key] = value
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid fields to update",
			}, "No valid fields to update")
		}

		updates["updated_at"] = time.Now()

		collection := database.GetCollection("workspaces")
		ctx := context.Background()

		err = collection.FindOne(ctx, bson.M{"_id": workspaceID}).Decode(&workspace)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		if workspace.OwnderID != userID {
			return http.Error(c, 403, fiber.Map{
				"error": "User can not update the workspace",
			}, "User can not update workspace")
		}

		updateResult, err := collection.UpdateByID(ctx, workspaceID, bson.M{
			"$set": updates,
		})

		if updateResult.ModifiedCount != 1 {
			return http.Error(c, 500, fiber.Map{
				"error": "No modified register",
			}, "No modified register")
		}

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error on update register",
			}, "Error to update register")
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

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to user auth")
		}

		userRole, err := services.GetWorkspaceUserRole(userID, workspaceID)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		collection := database.GetCollection("workspaces")
		ctx := context.Background()

		result := collection.FindOneAndUpdate(ctx, bson.M{
			"_id":      workspaceID,
			"owner_id": userID,
		}, bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		})

		if result.Err() != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Delete operation error",
			}, "Delete operation error")
		}

		err = services.Dispatcher.Dispatch(
			workspace.WorkspaceDeletedEvent{
				WorkspaceID: workspaceID,
				Ctx:         ctx,
			},
		)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to dispatch event",
			}, "Fail to dispatch event")
		}

		return http.Success(c, 200, "Workspace deleted")
	}
}
