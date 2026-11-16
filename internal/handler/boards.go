package handler

import (
	"context"
	"encoding/json"
	"kanban-management/internal/database"
	_ "kanban-management/internal/dto" // só para o swag resolver os tipos das anotações
	boardEvent "kanban-management/internal/events/board"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/pagination"
	"kanban-management/internal/search"
	"kanban-management/internal/services"
	"log/slog"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Campos que não podem ser alterados via PATCH
var boardProtectedFields = []string{
	"_id",
	"id",
	"project_id",
	"search_text",
	"created_at",
	"updated_at",
	"deleted_at",
}

// findActiveBoard busca um board que não foi deletado (soft delete)
func findActiveBoard(ctx context.Context, boardID bson.ObjectID) (*models.Board, error) {
	var board models.Board

	err := database.GetCollection("boards").FindOne(ctx, bson.M{
		"_id": boardID,
		"$or": services.NotDeleted(),
	}).Decode(&board)

	if err != nil {
		return nil, err
	}

	return &board, nil
}

// @Summary	Lista os boards de um projeto
// @Tags		boards
// @Accept		json
// @Produce	json
// @Param		id		path		string	true	"ID do projeto (ObjectID)"
// @Param		page	query		int		false	"Página (padrão 1)"							default(1)	minimum(1)
// @Param		limit	query		int		false	"Itens por página (padrão 20, máximo 100)"	default(20)	minimum(1)	maximum(100)
// @Param		q		query		string	false	"Busca no nome (2 a 100 caracteres)"
// @Success	200		{object}	dto.PageBoardsEnvelope
// @Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure	401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure	404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security	BearerAuth
// @Router		/board/project/{id} [get]
func GetBoards() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Param",
			}, "Missing Param")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		projectID, err := bson.ObjectIDFromHex(projectIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid project id",
			}, "Invalid project id")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionBoardView); err != nil {
			return respondAuthError(c, err)
		}

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		filter := bson.M{
			"project_id": projectID,
			"$or":        services.NotDeleted(),
		}

		terms, searching, err := searchTerms(c)

		if err != nil {
			return invalidCardResponse(c, search.InvalidQueryMessage)
		}

		if searching {
			filter["$and"] = search.RegexFilter("search_text", terms)["$and"]
		}

		boards, total, err := pagination.Fetch[models.Board](context.Background(), database.GetCollection("boards"), filter, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get project boards")
		}

		return http.Success(c, 200, pagination.NewPage(boards, params, total))
	}
}

// @Summary		Busca um board
// @Description	Retorna o board se o usuário for membro do projeto.
// @Tags			boards
// @Accept			json
// @Produce		json
// @Param			board_id	path		string	true	"ID do board (ObjectID)"
// @Success		200			{object}	dto.BoardEnvelope
// @Failure		400			{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401			{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		404			{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security		BearerAuth
// @Router			/board/{board_id} [get]
func GetBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardIDHex := c.Params("board_id")

		if boardIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing board id",
			}, "Missing board id")
		}

		boardID, err := bson.ObjectIDFromHex(boardIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid board id",
			}, "Invalid board id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		ctx := context.Background()

		board, err := findActiveBoard(ctx, boardID)

		if err != nil || board.ProjectID == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, *board.ProjectID, services.ActionBoardView); err != nil {
			return respondAuthError(c, err)
		}

		return http.Success(c, 200, board)
	}
}

// @Summary		Cria um board
// @Description	Exige manager ou admin do projeto.
// @Tags			boards
// @Accept			json
// @Produce		json
// @Param			id		path		string					true	"ID do projeto (ObjectID)"
// @Param			body	body		dto.CreateBoardRequest	true	"Dados do board"
// @Success		201		{object}	dto.BoardEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security		BearerAuth
// @Router			/board/project/{id} [post]
func CreateBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		projectIDHex := c.Params("id")

		if projectIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing Param",
			}, "Missing Param")
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

		if _, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionBoardManage); err != nil {
			return respondAuthError(c, err)
		}

		var board models.Board

		if err := c.BodyParser(&board); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid body",
			}, "Invalid body")
		}

		if board.Name == nil || *board.Name == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Board name is required",
			}, "Board name is required")
		}

		now := time.Now()

		board.SearchText = search.BuildText(*board.Name)
		board.ID = nil
		board.ProjectID = &projectID
		board.DeletedAt = nil
		board.CreatedAt, board.UpdatedAt = now, now

		for i := range board.Columns {
			board.Columns[i].CreatedAt = now
			board.Columns[i].UpdatedAt = now
		}

		for i := range board.Members {
			board.Members[i].CreatedAt = now
			board.Members[i].UpdatedAt = now
		}

		ctx := context.Background()
		collection := database.GetCollection("boards")

		result, err := collection.InsertOne(ctx, board)

		if err != nil {
			return http.ServerError(c, err, "Board not created")
		}

		if !result.Acknowledged {
			return http.ServerError(c, nil, "Error to create board")
		}

		if insertedID, ok := result.InsertedID.(bson.ObjectID); ok {
			board.ID = &insertedID
		}

		slog.Info("board.created", "board_id", board.ID, "project_id", projectID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 201, board)
	}
}

// @Summary		Edita um board
// @Description	Exige manager ou admin. Campos internos (_id, project_id, created_at...) são ignorados.
// @Tags			boards
// @Accept			json
// @Produce		json
// @Param			board_id	path		string					true	"ID do board (ObjectID)"
// @Param			body		body		dto.UpdateBoardRequest	true	"Campos a alterar"
// @Success		200			{object}	dto.StringEnvelope
// @Failure		400			{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401			{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403			{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404			{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security		BearerAuth
// @Router			/board/{board_id} [patch]
func PatchBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardIDHex := c.Params("board_id")

		if boardIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing board id",
			}, "Missing board id")
		}

		boardID, err := bson.ObjectIDFromHex(boardIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid board id",
			}, "Invalid board id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		var body map[string]any

		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid JSON",
			}, "Invalid JSON")
		}

		updates := make(map[string]any)

		for key, value := range body {
			if slices.Contains(boardProtectedFields, key) {
				continue
			}

			updates[key] = value
		}

		if len(updates) == 0 {
			return http.Error(c, 400, fiber.Map{
				"error": "No valid field to update",
			}, "No valid field to update")
		}

		ctx := context.Background()

		board, err := findActiveBoard(ctx, boardID)

		if err != nil || board.ProjectID == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, *board.ProjectID, services.ActionBoardManage); err != nil {
			return respondAuthError(c, err)
		}

		if name, ok := updates["name"].(string); ok {
			updates["search_text"] = search.BuildText(name)
		}

		updates["updated_at"] = time.Now()

		collection := database.GetCollection("boards")

		result, err := collection.UpdateByID(ctx, boardID, bson.M{
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

// @Summary		Deleta um board (soft delete)
// @Description	Exige manager ou admin. Deleta em cascata os cards e as labels do board.
// @Tags			boards
// @Accept			json
// @Produce		json
// @Param			board_id	path		string	true	"ID do board (ObjectID)"
// @Success		200			{object}	dto.StringEnvelope
// @Failure		400			{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401			{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403			{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404			{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security		BearerAuth
// @Router			/board/{board_id} [delete]
func DeleteBoard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		boardIDHex := c.Params("board_id")

		if boardIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing board id",
			}, "Missing board id")
		}

		boardID, err := bson.ObjectIDFromHex(boardIDHex)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid board id",
			}, "Invalid board id")
		}

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 401, fiber.Map{
				"error": "Fail to get user auth",
			}, "Fail to get user auth")
		}

		ctx := context.Background()

		board, err := findActiveBoard(ctx, boardID)

		if err != nil || board.ProjectID == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Not found",
			}, "Not found")
		}

		if _, err := services.AuthorizeProject(context.Background(), userID, *board.ProjectID, services.ActionBoardManage); err != nil {
			return respondAuthError(c, err)
		}

		now := time.Now()

		collection := database.GetCollection("boards")

		result, err := collection.UpdateByID(ctx, boardID, bson.M{
			"$set": bson.M{
				"deleted_at": now,
				"updated_at": now,
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Error to delete board")
		}

		if result.ModifiedCount == 0 {
			return http.ServerError(c, nil, "No board modified")
		}

		err = services.Dispatcher.Dispatch(
			boardEvent.BoardDeletedEvent{
				BoardID: boardID,
				Ctx:     ctx,
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Fail to dispatch board deleted event")
		}

		slog.Info("board.deleted", "board_id", boardID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, "deleted")
	}
}
