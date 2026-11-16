package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	projectEvent "kanban-management/internal/events/project"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/pagination"
	"kanban-management/internal/search"
	"kanban-management/internal/services"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Campos que o PATCH de projeto aceita (allowlist).
var projectPatchableFields = map[string]bool{
	"name":        true,
	"description": true,
	"status":      true,
}

func MyProjects() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Error to auth user",
			}, "Error to auth user")
		}

		ctx := context.Background()
		collection := database.GetCollection("projects")

		membersCursor, err := database.GetCollection("project_members").Find(ctx, bson.M{
			"user_id": userID,
			"$or":     services.NotDeleted(),
		})

		if err != nil {
			return http.ServerError(c, err, "Error to get projects")
		}

		defer membersCursor.Close(ctx)

		var memberships []models.ProjectMember

		if err = membersCursor.All(ctx, &memberships); err != nil {
			return http.ServerError(c, err, "Error to verify projects")
		}

		projectIDs := []bson.ObjectID{}

		for _, member := range memberships {
			if member.ProjectID != nil {
				projectIDs = append(projectIDs, *member.ProjectID)
			}
		}

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		filter := bson.M{
			"_id": bson.M{"$in": projectIDs},
			"$or": services.NotDeleted(),
		}

		terms, searching, err := searchTerms(c)

		if err != nil {
			return invalidCardResponse(c, search.InvalidQueryMessage)
		}

		if searching {
			filter["$and"] = search.RegexFilter("search_text", terms)["$and"]
		}

		projects, total, err := pagination.Fetch[models.Project](ctx, collection, filter, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Error to get projects")
		}

		return http.Success(c, 200, pagination.NewPage(projects, params, total))
	}
}

func CreateProject() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var project *models.Project

		if err := c.BodyParser(&project); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		userID, err := middleware.CurrentUserID(c)

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

		if !services.WorkspaceExists(project.WorkspaceID) {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		if _, err := services.GetWorkspaceUserRole(userID, project.WorkspaceID); err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		project.SearchText = search.BuildText(project.Name, project.Descritpion)
		project.Owner_id = userID
		project.Status = "active"

		now := time.Now()

		project.CreatedAt = now
		project.UpdatedAt = now

		ctx := context.Background()

		collection := database.GetCollection("projects")

		singleResult, err := collection.InsertOne(ctx, project)

		if err != nil {
			return http.ServerError(c, err, "Project not created")
		}

		if !singleResult.Acknowledged {
			return http.ServerError(c, nil, "Error to create project")
		}

		insertedID, ok := singleResult.InsertedID.(bson.ObjectID)

		if !ok {
			return http.ServerError(c, nil, "Error to create project")
		}

		project.ID = insertedID

		err = services.Dispatcher.Dispatch(
			&projectEvent.ProjectCreatedEvent{
				ProjectID: insertedID,
				UserID:    userID,
				Ctx:       ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch project created event")
		}

		slog.Info("project.created", "project_id", insertedID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 201, project)
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Error to user auth",
			}, "Error to user auth")
		}

		var body map[string]any

		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid JSON",
			}, "Invalid JSON")
		}

		updates := make(map[string]any)

		for key, value := range body {
			if projectPatchableFields[key] {
				updates[key] = value
			}
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid field to update",
			}, "No valid fields to update")
		}

		collection := database.GetCollection("projects")
		ctx := context.Background()

		if _, err := services.AuthorizeProject(ctx, userID, projectID, services.ActionProjectUpdate); err != nil {
			return respondAuthError(c, err)
		}

		// search_text acompanha nome e descrição (pega o valor atual do que não mudou).
		_, nameChanged := updates["name"].(string)
		_, descriptionChanged := updates["description"].(string)

		if nameChanged || descriptionChanged {
			var current models.Project

			if err := collection.FindOne(ctx, bson.M{"_id": projectID}).Decode(&current); err != nil {
				return http.ServerError(c, err, "Error to load project")
			}

			name, description := current.Name, current.Descritpion

			if v, ok := updates["name"].(string); ok {
				name = v
			}

			if v, ok := updates["description"].(string); ok {
				description = v
			}

			updates["search_text"] = search.BuildText(name, description)
		}

		updates["updated_at"] = time.Now()

		result, err := collection.UpdateByID(ctx, projectID, bson.M{
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		ctx := context.Background()
		collection := database.GetCollection("projects")

		if _, err := services.AuthorizeProject(ctx, userID, projectID, services.ActionProjectDelete); err != nil {
			return respondAuthError(c, err)
		}

		deleteResult, err := collection.UpdateByID(ctx, projectID, bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete project")
		}

		if deleteResult.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No project modified")
		}

		err = services.Dispatcher.Dispatch(
			projectEvent.ProjectDeletedEvent{
				ProjectID: projectID,
				Ctx:       ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch project deleted event")
		}

		slog.Info("project.deleted", "project_id", projectID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
