package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// CurrentUserID retorna o ID do usuário autenticado.
// Retorna erro se a rota não passou pelo AuthMiddleware.
func CurrentUserID(c *fiber.Ctx) (bson.ObjectID, error) {
	userID, ok := c.Locals("user_oid").(bson.ObjectID)

	if !ok {
		return bson.ObjectID{}, errors.New("user not found in context")
	}

	return userID, nil
}
