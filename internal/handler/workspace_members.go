package handler

import (
	"context"
	"kanban-management/internal/database"
	_ "kanban-management/internal/dto" // só para o swag resolver os tipos das anotações
	"kanban-management/internal/filters"
	"kanban-management/internal/http"
	"kanban-management/internal/middleware"
	"kanban-management/internal/models"
	"kanban-management/internal/pagination"
	"kanban-management/internal/services"
	"log/slog"
	"slices"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type SubjectMemberRequest struct {
	UserId string `json:"user_id"`
	Role   string `json:"role"`
}

// @Summary	Lista os workspaces em que o usuário é membro
// @Tags		workspace-members
// @Accept		json
// @Produce	json
// @Param		page	query		int	false	"Página (padrão 1)"							default(1)	minimum(1)
// @Param		limit	query		int	false	"Itens por página (padrão 20, máximo 100)"	default(20)	minimum(1)	maximum(100)
// @Success	200		{object}	dto.PageWorkspacesEnvelope
// @Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure	401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Security	BearerAuth
// @Router		/workspace/member [get]
func GetMyWorkspaces() fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to user from request auth",
			}, "Fail to user from request auth")
		}

		ctx := context.Background()

		membersCollection := database.GetCollection("workspaces_members")

		cursor, err := membersCollection.Find(ctx, bson.M{
			"user_id": userID,
			"$or":     services.NotDeleted(),
		})

		if err != nil {
			return http.ServerError(c, err, "Fail to get user workspaces")
		}

		defer cursor.Close(ctx)

		var workspaceMembers []models.WorkspaceMember

		if err = cursor.All(ctx, &workspaceMembers); err != nil {
			return http.ServerError(c, err, "Fail to get workspace members")
		}

		workspaceIDs := make([]bson.ObjectID, 0, len(workspaceMembers))

		for _, member := range workspaceMembers {
			workspaceIDs = append(workspaceIDs, member.WorkspaceID)
		}

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		workspaces, total, err := pagination.Fetch[models.Workspace](ctx, database.GetCollection("workspaces"), bson.M{
			"_id": bson.M{"$in": workspaceIDs},
			"$or": services.NotDeleted(),
		}, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get workspaces")
		}

		return http.Success(c, 200, pagination.NewPage(workspaces, params, total))
	}
}

// @Summary		Lista os membros de um workspace
// @Description	Qualquer membro pode ver.
// @Tags			workspace-members
// @Accept			json
// @Produce		json
// @Param			id		path		string	true	"ID do workspace (ObjectID)"
// @Param			page	query		int		false	"Página (padrão 1)"							default(1)	minimum(1)
// @Param			limit	query		int		false	"Itens por página (padrão 20, máximo 100)"	default(20)	minimum(1)	maximum(100)
// @Param			role	query		string	false	"Filtra por role(s): member, manager, admin (vírgula)"
// @Success		200		{object}	dto.PageWorkspaceMembersEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security		BearerAuth
// @Router			/workspace/member/{id} [get]
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to user from request auth",
			}, "Fail to user from request auth")
		}

		_, err = services.AuthorizeWorkspace(context.Background(), userID, workspaceID, services.ActionWorkspaceView)

		if err != nil {
			return respondAuthError(c, err)
		}

		params, err := pagination.Parse(c)

		if err != nil {
			return invalidCardResponse(c, pagination.InvalidParamsMessage)
		}

		filter := bson.M{
			"workspace_id": workspaceID,
			"$or":          services.NotDeleted(),
		}

		roles, err := filters.ParseRoles(c.Query("role"), services.UserRoles)

		if err != nil {
			return respondQueryError(c, err)
		}

		if roles != nil {
			filter["role"] = bson.M{"$in": roles}
		}

		members, total, err := pagination.Fetch[models.WorkspaceMember](context.Background(), database.GetCollection("workspaces_members"), filter, bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get members")
		}

		return http.Success(c, 200, pagination.NewPage(members, params, total))
	}
}

// @Summary		Adiciona um membro ao workspace
// @Description	Exige manager ou admin; só admin adiciona admin.
// @Tags			workspace-members
// @Accept			json
// @Produce		json
// @Param			id		path		string				true	"ID do workspace (ObjectID)"
// @Param			body	body		dto.MemberRequest	true	"Usuário e role"
// @Success		200		{object}	dto.WorkspaceMemberEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Failure		409		{object}	dto.ErrorResponse	"Já é membro"
// @Security		BearerAuth
// @Router			/workspace/member/{id} [post]
func AddMemberToWorkspace() fiber.Handler {
	return func(c *fiber.Ctx) error {
		workspaceIDHex := c.Params("id")

		if workspaceIDHex == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing workspace param",
			}, "Missing workspace param")
		}

		userID, err := middleware.CurrentUserID(c)

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

		userRole, err := services.AuthorizeWorkspace(context.Background(), userID, workspaceID, services.ActionWorkspaceManageMembers)

		if err != nil {
			return respondAuthError(c, err)
		}

		if !services.CanChangeMemberRole(userRole, "", subjectMemberRequest.Role) {
			return http.Error(c, 403, fiber.Map{
				"error": "Only admins can add admins",
			}, "Only admins can add admins")
		}

		collection := database.GetCollection("workspaces_members")
		ctx := context.Background()

		if !services.UserExists(ctx, userIDToAdd) {
			return http.Error(c, 404, fiber.Map{
				"error": "User not found",
			}, "User not found")
		}

		count, err := collection.CountDocuments(ctx, bson.M{
			"workspace_id": workspaceID,
			"user_id":      userIDToAdd,
			"$or":          services.NotDeleted(),
		})

		if err != nil {
			return http.ServerError(c, err, "Fail to check workspace member")
		}

		if count > 0 {
			return http.Error(c, 409, fiber.Map{
				"error": "User is already a workspace member",
			}, "User is already a workspace member")
		}

		var workspaceMember models.WorkspaceMember

		workspaceMember.UserID = userIDToAdd
		workspaceMember.WorkspaceID = workspaceID
		workspaceMember.Role = subjectMemberRequest.Role

		now := time.Now()

		workspaceMember.CreatedAt = now
		workspaceMember.UpdatedAt = now

		result, err := collection.InsertOne(ctx, workspaceMember)

		if err != nil {
			return http.ServerError(c, err, "Member not added")
		}

		if insertedID, ok := result.InsertedID.(bson.ObjectID); ok {
			workspaceMember.ID = insertedID
		}

		slog.Info("workspace.member_added", "workspace_id", workspaceID.Hex(), "member_id", userIDToAdd.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, workspaceMember)
	}
}

// @Summary		Muda a role de um membro do workspace
// @Description	Só admin promove, rebaixa ou mexe em admin.
// @Tags			workspace-members
// @Accept			json
// @Produce		json
// @Param			id		path		string				true	"ID do workspace (ObjectID)"
// @Param			body	body		dto.MemberRequest	true	"Usuário e nova role"
// @Success		200		{object}	dto.MessageEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Failure		409		{object}	dto.ErrorResponse	"O workspace precisa de pelo menos um admin"
// @Security		BearerAuth
// @Router			/workspace/member/{id} [patch]
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.ServerError(c, err, "Fail to user auth")
		}

		userRole, err := services.AuthorizeWorkspace(context.Background(), userID, workspaceID, services.ActionWorkspaceManageMembers)

		if err != nil {
			return respondAuthError(c, err)
		}

		var subjectMemberRequest SubjectMemberRequest

		if err = c.BodyParser(&subjectMemberRequest); err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Missing body",
			}, "Missing body")
		}

		if subjectMemberRequest.UserId == "" || subjectMemberRequest.Role == "" {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid user id or role on body",
			}, "Invalid user id or role on body")
		}

		if !slices.Contains(services.UserRoles, subjectMemberRequest.Role) {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid role on body",
			}, "Invalid role on body")
		}

		subjectUserID, err := bson.ObjectIDFromHex(subjectMemberRequest.UserId)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Invalid member user id",
			}, "Invalid member user id")
		}

		collection := database.GetCollection("workspaces_members")
		ctx := context.Background()

		var target models.WorkspaceMember

		err = collection.FindOne(ctx, bson.M{
			"workspace_id": workspaceID,
			"user_id":      subjectUserID,
			"$or":          services.NotDeleted(),
		}).Decode(&target)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace member not found",
			}, "Workspace member not found")
		}

		if !services.CanChangeMemberRole(userRole, target.Role, subjectMemberRequest.Role) {
			return http.Error(c, 403, fiber.Map{
				"error": "Only admins can manage admins",
			}, "Only admins can manage admins")
		}

		if subjectMemberRequest.Role != services.RoleAdmin {
			if err := services.EnsureNotLastAdmin(ctx, "workspaces_members", "workspace_id", workspaceID, target.Role); err != nil {
				return respondLastAdminError(c, err, "Workspace must have at least one admin")
			}
		}

		_, err = collection.UpdateByID(ctx, target.ID, bson.M{
			"$set": bson.M{
				"role":       subjectMemberRequest.Role,
				"updated_at": time.Now(),
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Member not updated")
		}

		return http.Success(c, 200, fiber.Map{"message": "Member updated"})
	}
}

// @Summary	Remove um membro do workspace
// @Tags		workspace-members
// @Accept		json
// @Produce	json
// @Param		id		path		string					true	"ID do workspace (ObjectID)"
// @Param		body	body		dto.RemoveMemberRequest	true	"Usuário a remover"
// @Success	200		{object}	dto.MessageEnvelope
// @Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure	401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure	403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure	404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Failure	409		{object}	dto.ErrorResponse	"O workspace precisa de pelo menos um admin"
// @Security	BearerAuth
// @Router		/workspace/member/{id} [delete]
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 400, fiber.Map{
				"error": "Fail to user auth",
			}, "Fail to user auth")
		}

		userRole, err := services.AuthorizeWorkspace(context.Background(), userID, workspaceID, services.ActionWorkspaceManageMembers)

		if err != nil {
			return respondAuthError(c, err)
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

		var target models.WorkspaceMember

		err = collection.FindOne(ctx, bson.M{
			"workspace_id": workspaceID,
			"user_id":      subjectUserId,
			"$or":          services.NotDeleted(),
		}).Decode(&target)

		if err != nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Workspace member not found",
			}, "Workspace member not found")
		}

		if !services.CanChangeMemberRole(userRole, target.Role, "") {
			return http.Error(c, 403, fiber.Map{
				"error": "Only admins can manage admins",
			}, "Only admins can manage admins")
		}

		if err := services.EnsureNotLastAdmin(ctx, "workspaces_members", "workspace_id", workspaceID, target.Role); err != nil {
			return respondLastAdminError(c, err, "Workspace must have at least one admin")
		}

		now := time.Now()

		_, err = collection.UpdateByID(ctx, target.ID, bson.M{
			"$set": bson.M{
				"deleted_at": now,
				"updated_at": now,
			},
		})

		if err != nil {
			return http.ServerError(c, err, "Member not deleted")
		}

		slog.Info("workspace.member_removed", "workspace_id", workspaceID.Hex(), "member_id", subjectUserId.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, fiber.Map{"message": "Member Deleted"})
	}
}
