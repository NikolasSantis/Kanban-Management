package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	userEvents "kanban-management/internal/events/user"
	"kanban-management/internal/http"
	"kanban-management/internal/models"
	"kanban-management/internal/services"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIDHex := c.Params("id")

		if userIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Params",
			}, "Missing Params")
		}

		userID, err := bson.ObjectIDFromHex(userIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid param",
			}, "Invalid Param")
		}

		var user models.User

		collection := database.GetCollection("users")
		ctx := context.Background()

		err = collection.FindOne(ctx, bson.M{
			"_id": userID,
		}).Decode(&user)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "User not found",
			}, "User not found")
		}

		return http.Success(c, 200, fiber.Map{
			"id":                user.ID,
			"name":              user.Name,
			"email":             user.Email,
			"email_verified_at": user.EmailVerifiedAt,
			"expiration_date":   user.ExpirationDate,
			"created_at":        user.CreatedAt,
			"updated_at":        user.UpdatedAt,
		})
	}
}

func PatchUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIDHex := c.Params("id")

		if userIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Param",
			}, "Missing Param")
		}

		userID, err := bson.ObjectIDFromHex(userIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid param",
			}, "Invalid Param")
		}

		var body map[string]any

		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid JSON",
			}, "Invalid JSON")
		}

		allowedFields := map[string]bool{
			"name":  true,
			"email": true,
		}

		updates := make(map[string]any)

		for key, value := range body {
			if allowedFields[key] {
				updates[key] = value
			}
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid fields to update",
			}, "No valid fields to update")
		}

		updates["updated_at"] = time.Now()

		collection := database.GetCollection("users")
		ctx := context.Background()

		updateResults, err := collection.UpdateByID(
			ctx,
			userID,
			bson.M{
				"$set": updates,
			},
		)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Update failed",
			}, "Update Failed")
		}

		return http.Success(c, 200, updateResults)
	}
}

func DeleteUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userIDHex := c.Params("id")

		if userIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Param",
			}, "Missing Param")
		}

		if c.Locals("user_id").(string) != userIDHex {
			return http.Error(c, 403, fiber.Map{
				"error": "Unathorized",
			}, "Unauthorized")
		}

		userID, err := bson.ObjectIDFromHex(userIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid param",
			}, "Invalid param")
		}

		var user models.User

		collection := database.GetCollection("users")
		ctx := context.Background()

		err = collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "User not found",
			}, "User not found")
		}

		deleteResults := collection.FindOneAndUpdate(ctx, bson.M{
			"_id": userID,
		}, bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		})

		if deleteResults.Err() != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Delete operation error",
			}, "Delete operation error")
		}

		err = services.Dispatcher.Dispatch(
			userEvents.UserDeletedEvent{
				UserID: userID,
				Ctx:    ctx,
			},
		)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error to dispatch event",
				"err":   err,
			}, "Error to dispatch event")
		}

		return http.Success(c, 200, fiber.Map{
			"message": "User Deleted",
		})
	}
}
