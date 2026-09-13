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

type SubjectProjectMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func GetMembersOnProject() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing project param",
			}, "Missing project param")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project param",
			}, "Invalid project param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from request auth",
			}, "Fail to get user from request auth")
		}

		userRole, err := services.GetProjectUserRole(userID, projectID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project not found",
			}, "Project not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User without permission")
		}

		collection := database.GetCollection("project_members")
		ctx := context.Background()

		cursor, err := collection.Find(ctx, bson.M{
			"project_id": &projectID,
			"$or": []bson.M{
				{
					"deleted_at": bson.M{
						"$exists": false,
					},
				},
				{
					"deleted_at": nil,
				},
			},
		})

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to get project members",
			}, "Fail to get project members")
		}

		defer cursor.Close(ctx)

		var members []models.ProjectMember

		if err := cursor.All(ctx, &members); err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail to decode project members",
			}, "Fail to decode project members")
		}

		return http.Success(c, 200, members)
	}
}

func AddMemberToProject() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing project param",
			}, "Missing project param")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project param",
			}, "Invalid project param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from request auth",
			}, "Fail to get user from request auth")
		}

		var request SubjectProjectMemberRequest

		if err := c.BodyParser(&request); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		if request.UserID == "" || request.Role == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid user id or role",
			}, "Invalid user id or role")
		}

		if !slices.Contains(services.UserRoles, request.Role) {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid role",
			}, "Invalid role")
		}

		userIDToAdd, err := bson.ObjectIDFromHex(request.UserID)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid member user id",
			}, "Invalid member user id")
		}

		userRole, err := services.GetProjectUserRole(userID, projectID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project not found",
				"e:":    err.Error(),
			}, "Project not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User without permission")
		}

		collection := database.GetCollection("project_members")
		ctx := context.Background()

		now := time.Now()

		var existingMember models.ProjectMember

		err = collection.FindOne(ctx, bson.M{
			"project_id": &projectID,
			"user_id":    userIDToAdd,
		}).Decode(&existingMember)

		if err == nil {
			if existingMember.DeletedAt == nil {
				return http.Error(c, 409, fiber.Map{
					"error": "User is already a project member",
				}, "User is already a project member")
			}

			_, err = collection.UpdateOne(
				ctx,
				bson.M{
					"_id": existingMember.ID,
				},
				bson.M{
					"$set": bson.M{
						"role":       request.Role,
						"updated_at": now,
					},
					"$unset": bson.M{
						"deleted_at": "",
					},
				},
			)

			if err != nil {
				return http.Error(c, 500, fiber.Map{
					"error": "Failed to reactivate project member",
				}, "Failed to reactivate project member")
			}

			existingMember.Role = &request.Role
			existingMember.UpdatedAt = now
			existingMember.DeletedAt = nil

			return http.Success(c, 200, existingMember)
		}

		member := models.ProjectMember{
			UserID:    &userIDToAdd,
			ProjectID: &projectID,
			Role:      &request.Role,
			CreatedAt: now,
			UpdatedAt: now,
		}

		result, err := collection.InsertOne(ctx, member)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Member not added",
			}, "Member not added")
		}

		member.ID = result.InsertedID.(*bson.ObjectID)

		return http.Success(c, 201, member)
	}
}

func PatchProjectMember() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing project param",
			}, "Missing project param")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project param",
			}, "Invalid project param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from auth",
			}, "Fail to get user from auth")
		}

		userRole, err := services.GetProjectUserRole(userID, projectID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project not found",
			}, "Project not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User without permission")
		}

		var request SubjectProjectMemberRequest

		if err := c.BodyParser(&request); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		if request.UserID == "" || request.Role == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid user id or role",
			}, "Invalid user id or role")
		}

		if !slices.Contains(services.UserRoles, request.Role) {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid role",
			}, "Invalid role")
		}

		subjectUserID, err := bson.ObjectIDFromHex(request.UserID)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid member user id",
			}, "Invalid member user id")
		}

		collection := database.GetCollection("project_members")
		ctx := context.Background()

		result := collection.FindOneAndUpdate(
			ctx,
			bson.M{
				"project_id": &projectID,
				"user_id":    subjectUserID,
				"$or": []bson.M{
					{
						"deleted_at": bson.M{
							"$exists": false,
						},
					},
					{
						"deleted_at": nil,
					},
				},
			},
			bson.M{
				"$set": bson.M{
					"role":       request.Role,
					"updated_at": time.Now(),
				},
			},
		)

		if result.Err() != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project member not found",
			}, "Project member not found")
		}

		return http.Success(c, 200, fiber.Map{
			"message": "Member updated",
		})
	}
}

func DeleteProjectMember() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing project param",
			}, "Missing project param")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project param",
			}, "Invalid project param")
		}

		userID, err := bson.ObjectIDFromHex(c.Locals("user_id").(string))

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from auth",
			}, "Fail to get user from auth")
		}

		userRole, err := services.GetProjectUserRole(userID, projectID)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project not found",
			}, "Project not found")
		}

		if !slices.Contains(services.UserRolesCanUpdate, userRole) {
			return http.Error(c, 403, fiber.Map{
				"error": "User without permission",
			}, "User without permission")
		}

		var request SubjectProjectMemberRequest

		if err := c.BodyParser(&request); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		if request.UserID == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid user id",
			}, "Invalid user id")
		}

		subjectUserID, err := bson.ObjectIDFromHex(request.UserID)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid member user id",
			}, "Invalid member user id")
		}

		collection := database.GetCollection("project_members")
		ctx := context.Background()

		now := time.Now()

		result, err := collection.UpdateOne(
			ctx,
			bson.M{
				"project_id": &projectID,
				"user_id":    subjectUserID,
				"$or": []bson.M{
					{
						"deleted_at": bson.M{
							"$exists": false,
						},
					},
					{
						"deleted_at": nil,
					},
				},
			},
			bson.M{
				"$set": bson.M{
					"deleted_at": now,
					"updated_at": now,
				},
			},
		)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Member not deleted",
			}, "Member not deleted")
		}

		if result.MatchedCount == 0 {
			return http.Error(c, 404, fiber.Map{
				"error": "Project member not found",
			}, "Project member not found")
		}

		return http.Success(c, 200, fiber.Map{
			"message": "Member deleted",
		})
	}
}
