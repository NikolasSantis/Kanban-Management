package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/models"

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

		for field, value := range body {
			if allowedFields[field] {
				updates[field] = value
			}
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid fields to update",
			}, "No valid fields to update")
		}

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

		deleteResults, err := collection.DeleteOne(ctx, bson.M{
			"_id": userID,
		})

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Fail on delete operation",
				"desc":  err.Error(),
			}, "Fail on delete operation")
		}

		if deleteResults.DeletedCount != 1 {
			return http.Error(c, 500, fiber.Map{
				"error": "User not deleted",
			}, "User not deleted")
		}

		return http.Success(c, 200, fiber.Map{
			"message": "User Deleted",
		})
	}
}
