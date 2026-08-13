package middleware

import (
	"strings"

	"kanban-management/internal/auth"
	"kanban-management/internal/http"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")

	if authHeader == "" {
		return http.Error(c, 401, fiber.Map{
			"error": "Authorization header is required",
		}, "Authorization header is required")
	}

	parts := strings.Split(authHeader, " ")

	if len(parts) != 2 || parts[0] != "Bearer" {
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

	c.Locals("user_id", userID)

	return c.Next()
}
