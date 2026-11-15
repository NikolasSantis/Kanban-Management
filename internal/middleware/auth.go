package middleware

import (
	"context"
	"strings"

	"kanban-management/internal/auth"
	"kanban-management/internal/database"
	"kanban-management/internal/http"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func AuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")

	if authHeader == "" {
		return http.Error(c, 401, fiber.Map{
			"error": "Authorization header is required",
		}, "Authorization header is required")
	}

	parts := strings.Fields(authHeader)

	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return http.Error(c, 401, fiber.Map{
			"error": "Invalid authorization header",
		}, "Invalid authorization header")
	}

	tokenString := parts[1]

	token, err := auth.ValidateToken(tokenString)

	if err != nil || !token.Valid {
		return http.Error(c, 401, fiber.Map{
			"error": "Invalid token",
		}, "Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)

	if !ok {
		return http.Error(c, 401, fiber.Map{
			"error": "Invalid token claims",
		}, "Invalid token claims")
	}

	userID, ok := claims["user_id"].(string)

	if !ok {
		return http.Error(c, 401, fiber.Map{
			"error": "User ID not found in token",
		}, "User ID not found in token")
	}

	userOID, err := bson.ObjectIDFromHex(userID)

	if err != nil {
		return http.Error(c, 401, fiber.Map{
			"error": "Invalid token",
		}, "Invalid token")
	}

	err = database.GetCollection("users").FindOne(
		context.Background(),
		bson.M{
			"_id": userOID,
			"$or": []bson.M{
				{"deleted_at": bson.M{"$exists": false}},
				{"deleted_at": nil},
			},
		},
		options.FindOne().SetProjection(bson.M{"_id": 1, "deleted_at": 1}),
	).Err()

	if err != nil {
		return http.Error(c, 401, fiber.Map{
			"error": "User not found or inactive",
		}, "User not found or inactive")
	}

	c.Locals("user_id", userOID.Hex())
	c.Locals("user_oid", userOID)

	return c.Next()
}
