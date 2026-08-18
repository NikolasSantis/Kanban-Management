package handler

import (
	"context"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/models"
	"kanban-management/internal/services"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubjectMemberRequest struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
}

func GetMyWorkspaces() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to user from request auth",
			}, "Fail to user from request auth")
		}

		ctx := context.Background()

		membersCollection := database.GetCollection("workspaces_members")

		cursor, err := membersCollection.Find(ctx, bson.M{
			"user_id": userID,
			"$or": []bson.M{
				{
					"deleted_at": bson.M{
						"$exists": false,
					},
				},
				{
					"deleted_at": time.Time{},
				},
			},
		})

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to get user workspaces",
			}, "Fail to get user workspaces")
		}

		defer cursor.Close(ctx)

		var workspaceMembers []models.WorkspaceMember

		if err = cursor.All(ctx, &workspaceMembers); err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to get workspace members",
			}, "Fail to get workspace members")
		}

		workspaceIDs := make([]bson.ObjectID, 0, len(workspaceMembers))

		for _, member := range workspaceMembers {
			workspaceIDs = append(workspaceIDs, member.WorkspaceID)
		}

		var workspaces []models.Workspace

		if len(workspaceIDs) > 0 {
			workspacesCollection := database.GetCollection("workspaces")

			cursor, err := workspacesCollection.Find(ctx, bson.M{
				"_id": bson.M{
					"$in": workspaceIDs,
				},
			})

			if err != nil {
				return http.Error(c, 500, fiber.Map{
					"error": "Fail to get workspaces",
				}, "Fail to get workspaces")
			}

			defer cursor.Close(ctx)

			if err = cursor.All(ctx, &workspaces); err != nil {
				return http.Error(c, 500, fiber.Map{
					"error": "Fail to decode workspaces",
				}, "Fail to decode workspaces")
			}
		}

		return http.Success(c, 200, workspaces)
	}
}

func GetMembersOnWorkspace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIDHex := c.Params("id")

		if workspaceIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing workspace param",
			}, "Missing workspace param")
		}

		workspaceID, err := bson.ObjectIDFromHex(workspaceIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid workspace param",
			}, "Invalid workspace param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to user from request auth",
			}, "Fail to user from request auth")
		}

		userRole, err := services.GetWorkspaceUserRole(userID, workspaceID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User without permission")
		}

		var workspaceMembers []models.WorkspaceMember

		collection := database.GetCollection("workspaces_members")
		ctx := context.Background()

		cursor, err := collection.Find(ctx, bson.M{
			"workspace_id": workspaceID,
			"$or": []bson.M{
				{
					"deleted_at": bson.M{
						"$exists": false,
					},
				},
				{
					"deleted_at": time.Time{},
				},
			},
		})

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace members not found",
			}, "Workspace members not found")
		}

		defer cursor.Close(ctx)

		if err = cursor.All(ctx, &workspaceMembers); err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to get members",
			}, "Fail to get members")
		}

		return http.Success(c, 200, workspaceMembers)
	}
}

func AddMemberToWorkspace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIDHex := c.Params("id")

		if workspaceIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing workspace param",
			}, "Missing workspace param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to user from request auth",
			}, "Fail to user from request auth")
		}

		workspaceID, err := bson.ObjectIDFromHex(workspaceIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Fail to valid workspace id",
			}, "Fail to valid workspace id")
		}

		var subjectMemberRequest SubjectMemberRequest

		if err := c.BodyParser(&subjectMemberRequest); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing body",
			}, "Missing body")
		}

		if subjectMemberRequest.UserId == "" || subjectMemberRequest.Role == "" {
			return http.Error(c, 403, fiber.Map{
				"error": "Invalid user id or role on body",
			}, "Invalid user id or role on body")
		}

		if !slices.Contains(services.UserRoles, subjectMemberRequest.Role) {
			return http.Error(c, 403, fiber.Map{
				"error": "Invalid role on body",
			}, "Invalid role on body")
		}

		userIDToAdd, err := bson.ObjectIDFromHex(subjectMemberRequest.UserId)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Error on new member id",
			}, "Error on new member id")
		}

		userRole, err := services.GetWorkspaceUserRole(userID, workspaceID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User without permission")
		}

		var workspaceMember models.WorkspaceMember

		workspaceMember.UserID = userIDToAdd
		workspaceMember.WorkspaceID = workspaceID
		workspaceMember.Role = subjectMemberRequest.Role

		now := time.Now()

		workspaceMember.CreatedAt = now
		workspaceMember.UpdatedAt = now

		collection := database.GetCollection("workspaces_members")
		ctx := context.Background()

		result, err := collection.InsertOne(ctx, workspaceMember)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Member not added",
			}, "Member not added")
		}

		workspaceMember.ID = result.InsertedID.(bson.ObjectID)

		return http.Success(c, 200, workspaceMember)
	}
}

func PatchWorkspaceMember() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIdHex := c.Params("id")

		if workspaceIdHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing workspace param",
			}, "Missing workspace param")
		}

		workspaceID, err := bson.ObjectIDFromHex(workspaceIdHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid workspace param",
			}, "Invalid workspace param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to user auth")
		}

		userRole, err := services.GetWorkspaceUserRole(userID, workspaceID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User wihtout permission")
		}

		var subjectMemberRequest SubjectMemberRequest

		if err = c.BodyParser(&subjectMemberRequest); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing body",
			}, "Missing body")
		}

		if subjectMemberRequest.UserId == "" || subjectMemberRequest.Role == "" {
			return http.Error(c, 403, fiber.Map{
				"error": "Invalid user id or role on body",
			}, "Invalid user id or role on body")
		}

		collection := database.GetCollection("workspaces_members")
		ctx := context.Background()

		result := collection.FindOneAndReplace(ctx, bson.M{
			"workspace_id": workspaceID,
			"user_id":      subjectMemberRequest.UserId,
		}, bson.M{
			"$set": bson.M{
				"role":       subjectMemberRequest.Role,
				"updated_at": time.Now(),
			},
		})

		if result.Err() != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Member not updated",
				"msg":   result.Err(),
			}, "Member not updated")
		}

		return http.Success(c, 200, fiber.Map{"message": "Member updated"})
	}
}

func DeleteWorkspaceMember() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIDHex := c.Params("id")

		if workspaceIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing workspace param",
			}, "Missing workspace param")
		}

		workspaceID, err := bson.ObjectIDFromHex(workspaceIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid workspace param",
			}, "Invalid workspace param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to user auth")
		}

		userRole, err := services.GetWorkspaceUserRole(userID, workspaceID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace not found",
			}, "Workspace not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User wihtout permission")
		}

		var subjectMemberRequest SubjectMemberRequest

		if err = c.BodyParser(&subjectMemberRequest); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing body",
			}, "Missing body")
		}

		if subjectMemberRequest.UserId == "" {
			return http.Error(c, 403, fiber.Map{
				"error": "Invalid user id on body",
			}, "Invalid user id on body")
		}

		collection := database.GetCollection("workspaces_members")
		ctx := context.Background()

		subjectUserId, err := bson.ObjectIDFromHex(subjectMemberRequest.UserId)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Error to convert subject member id",
			}, "Error to convert subject member ")
		}

		result := collection.FindOneAndUpdate(ctx, bson.M{
			"workspace_id": workspaceID,
			"user_id":      subjectUserId,
		}, bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		})

		if result.Err() != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Member not deleted",
			}, "Member not deleted")
		}

		return http.Success(c, 200, fiber.Map{"message": "Member Deleted"})
	}
}
