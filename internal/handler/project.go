package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/models"
	"kanban-management/internal/services"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func MyProjects() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Error to auth user",
			}, "Error to auth user")
		}

		ctx := context.Background()
		collection := database.GetCollection("projects")

		cursor, err := collection.Find(ctx, bson.M{
			"owner_id": userID,
		})

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Error to get projects",
			}, "Error to get projects")
		}

		defer cursor.Close(ctx)

		var projects []models.Project

		if err = cursor.All(ctx, &projects); err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error to verify projects",
			}, "Error to verify projects")
		}

		return http.Success(c, 200, projects)
	}
}

func CreateProject() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var project models.Project

		if err := c.BodyParser(&project); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		workspaceIDHex := project.WorkspaceID.Hex()

		project.WorkspaceID, err = bson.ObjectIDFromHex(workspaceIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid workspace id",
			}, "Invalid workspace ID")
		}

		if services.WorkspaceExists(project.WorkspaceID) {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		project.Owner_id = userID
		project.Status = "active"

		now := time.Now()

		project.CreatedAt, project.UpdatedAt = now, now

		ctx := context.Background()
		collection := database.GetCollection("projects")

		singleResult, err := collection.InsertOne(ctx, project)

		if singleResult.Acknowledged == false {
			return http.Error(c, 500, fiber.Map{
				"error": "Error to create project",
			}, "Error to create project")
		}

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Project not created",
			}, "Project not created")
		}

		return http.Success(c, 200, "Project creatd")
	}
}

func PatchProject() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing project",
			}, "Missing project")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project",
			}, "Invalid project")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Error to user auth",
			}, "Error to user auth")
		}

		var project models.Project
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
				"error": "No valid field to update",
			}, "No valid fields to update")
		}

		collection := database.GetCollection("projects")
		ctx := context.Background()

		search := collection.FindOne(ctx, bson.M{
			"_id": projectID,
		}).Decode(&project)

		if search != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if project.Owner_id != userID {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		updates["updated_at"] = time.Now()

		result, err := collection.UpdateByID(ctx, projectID, bson.M{
			"$set": updates,
		})

		if result.ModifiedCount == 0 {
			return http.Error(c, 500, fiber.Map{
				"error": "Not data updated",
			}, "Not data updated")
		}

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error to update",
			}, "Error to update")
		}

		return http.Success(c, 200, "Updated")
	}
}

func DeleteProject() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing param",
			}, "Missin param")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project id",
			}, "Invalid project id")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		ctx := context.Background()
		collection := database.GetCollection("projects")

		var requestedProject models.Project

		search := collection.FindOne(ctx, bson.M{
			"_id":      projectID,
			"owner_id": userID,
		}).Decode(&requestedProject)

		if search != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not Found")
		}

		if requestedProject.Owner_id != userID {
			return http.Error(c, 403, fiber.Map{
				"error": "Forbidden",
			}, "Forbidden")
		}

		deleteResult, err := collection.UpdateByID(ctx, projectID, bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		})

		if deleteResult.ModifiedCount == 0 {
			return http.Error(c, 500, fiber.Map{
				"error": "No project modified",
			}, "No project modified")
		}

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error to delete project",
			}, "Error to delete project")
		}

		return http.Success(c, 200, "deleted")
	}
}
