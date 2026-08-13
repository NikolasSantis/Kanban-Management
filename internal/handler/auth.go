package handler

import (
	"context"
	"kanban-management/internal/auth"
	"kanban-management/internal/database"
	"kanban-management/internal/http"
	"kanban-management/internal/models"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func Login() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var login LoginRequest

		if err := c.BodyParser(&login); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid format data"},
				"Invalid format data")
		}

		var user models.User

		collection := database.GetCollection("users")
		ctx := context.Background()

		err := collection.FindOne(
			ctx,
			bson.M{"email": login.Email},
		).Decode(&user)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Invalid email or password",
			}, "Invalid email or password")
		}

		err = bcrypt.CompareHashAndPassword(
			[]byte(user.Password),
			[]byte(login.Password),
		)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Invalid email or password",
			}, "Invalid email or password")
		}

		token, err := auth.GenerateToken(user.ID.Hex())

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error generating token",
			}, "Error generating token")
		}

		return http.Success(c, 200, fiber.Map{
			"token": token,
		})
	}
}

func RegisterNewUser() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var user models.User

		if err := c.BodyParser(&user); err != nil {
			return http.Error(c, 400, fiber.Map{"error": "Invalid Data"}, "Invalid Data")
		}

		hashedPassword, err := bcrypt.GenerateFromPassword(
			[]byte(user.Password),
			bcrypt.DefaultCost,
		)

		if err != nil {
			return http.Error(c, 500, fiber.Map{
				"error": "Error to hash password",
			}, "Error to hash password")
		}

		user.Password = string(hashedPassword)

		now := time.Now()

		user.ExpirationDate = now.AddDate(0, 0, 30)
		user.CreatedAt = now
		user.UpdatedAt = now

		collection := database.GetCollection("users")
		ctx := context.Background()

		matchedRows, err := collection.CountDocuments(ctx, bson.M{"email": user.Email})

		if err != nil {
			return http.Error(c, 500, fiber.Map{"error": err.Error()}, "Error to check email")
		}

		if matchedRows != 0 {
			return http.Error(c, 409, fiber.Map{"error": "User already exists"}, "User already exists")
		}

		_, err = collection.InsertOne(ctx, user)

		if err != nil {
			return http.Error(c, 500, fiber.Map{"error": "Error to register user"}, "Error to register user")
		}

		return http.Success(c, 200, fiber.Map{"success": true})
	}
}

func Me() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID := c.Locals("user_id").(string)

		objectID, err := bson.ObjectIDFromHex(userID)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Invalid user ID",
			}, "Invalid user ID")
		}

		var user models.User

		collection := database.GetCollection("users")
		ctx := context.Background()

		err = collection.FindOne(ctx, bson.M{
			"_id": objectID,
		}).Decode(&user)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "User not found",
			}, "User not found")
		}

		return http.Success(c, 200, fiber.Map{
			"id":              user.ID,
			"name":            user.Name,
			"email":           user.Email,
			"expiration_date": user.ExpirationDate,
			"created_at":      user.CreatedAt,
		})
	}
}
