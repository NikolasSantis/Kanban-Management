package handler

import (
	"context"
	"kanban-management/internal/auth"
	"kanban-management/internal/database"
	_ "kanban-management/internal/dto" // só para o swag resolver os tipos das anotações
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/services"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// @Summary		Faz login
// @Description	Devolve o token JWT; use no botão Authorize como: Bearer <token>.
// @Tags			auth
// @Accept			json
// @Produce		json
// @Param			body	body		dto.LoginRequest	true	"Credenciais"
// @Success		200		{object}	dto.TokenEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"E-mail ou senha incorretos (ou conta deletada)"
// @Router			/auth/login [post]
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
			bson.M{
				"email": login.Email,
				"$or":   services.NotDeleted(),
			},
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
			return http.ServerError(c, err, "Error generating token")
		}

		return http.Success(c, 200, fiber.Map{
			"token": token,
		})
	}
}

// @Summary	Cadastra um usuário
// @Tags		auth
// @Accept		json
// @Produce	json
// @Param		body	body		dto.RegisterRequest	true	"Dados do usuário"
// @Success	200		{object}	dto.RegisterEnvelope
// @Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure	409		{object}	dto.ErrorResponse	"E-mail já cadastrado"
// @Failure	500		{object}	dto.ErrorResponse	"Erro interno"
// @Router		/auth/register [post]
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
			return http.ServerError(c, err, "Error to hash password")
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
			return http.ServerError(c, err, "Error to check email")
		}

		if matchedRows != 0 {
			return http.Error(c, 409, fiber.Map{"error": "User already exists"}, "User already exists")
		}

		insertResult, err := collection.InsertOne(ctx, user)

		if err != nil {
			return http.ServerError(c, err, "Error to register user")
		}

		if insertedID, ok := insertResult.InsertedID.(bson.ObjectID); ok {
			slog.Info("user.registered", "user_id", insertedID.Hex())
		}

		return http.Success(c, 200, fiber.Map{"success": true})
	}
}

// @Summary	Dados do usuário logado
// @Tags		auth
// @Accept		json
// @Produce	json
// @Success	200	{object}	dto.UserEnvelope
// @Failure	401	{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure	404	{object}	dto.ErrorResponse	"Usuário não encontrado"
// @Security	BearerAuth
// @Router		/auth/me [get]
func Me() fiber.Handler {
	return func(c *fiber.Ctx) error {
		objectID, err := middleware.CurrentUserID(c)

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
