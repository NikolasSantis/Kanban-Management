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

type SubjectProjectMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// @Summary		Lista os membros de um projeto
// @Description	Qualquer membro pode ver.
// @Tags			project-members
// @Accept			json
// @Produce		json
// @Param			id		path		string	true	"ID do projeto (ObjectID)"
// @Param			page	query		int		false	"Página (padrão 1)"							default(1)	minimum(1)
// @Param			limit	query		int		false	"Itens por página (padrão 20, máximo 100)"	default(20)	minimum(1)	maximum(100)
// @Param			role	query		string	false	"Filtra por role(s): member, manager, admin (vírgula)"
// @Success		200		{object}	dto.PageProjectMembersEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Security		BearerAuth
// @Router			/project/member/{id} [get]
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from request auth",
			}, "Fail to get user from request auth")
		}

		_, err = services.AuthorizeProject(context.Background(), userID, projectID, services.ActionProjectView)

		if err != nil {
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

		roles, err := filters.ParseRoles(c.Query("role"), services.UserRoles)

		if err != nil {
			return respondQueryError(c, err)
		}

		if roles != nil {
			filter["role"] = bson.M{"$in": roles}
		}

		members, total, err := pagination.Fetch[models.ProjectMember](context.Background(), database.GetCollection("project_members"), filter, bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}, params)

		if err != nil {
			return http.ServerError(c, err, "Fail to get project members")
		}

		return http.Success(c, 200, pagination.NewPage(members, params, total))
	}
}

// @Summary		Adiciona um membro ao projeto
// @Description	Exige manager ou admin; só admin adiciona admin. Reativa quem tinha saído (200).
// @Tags			project-members
// @Accept			json
// @Produce		json
// @Param			id		path		string				true	"ID do projeto (ObjectID)"
// @Param			body	body		dto.MemberRequest	true	"Usuário e role"
// @Success		201		{object}	dto.ProjectMemberEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Failure		409		{object}	dto.ErrorResponse	"Já é membro"
// @Security		BearerAuth
// @Router			/project/member/{id} [post]
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

		userID, err := middleware.CurrentUserID(c)

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

		userRole, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionProjectManageMembers)

		if err != nil {
			return respondAuthError(c, err)
		}

		if !services.CanChangeMemberRole(userRole, "", request.Role) {
			return http.Error(c, 403, fiber.Map{
				"error": "Only admins can add admins",
			}, "Only admins can add admins")
		}

		collection := database.GetCollection("project_members")
		ctx := context.Background()

		if !services.UserExists(ctx, userIDToAdd) {
			return http.Error(c, 404, fiber.Map{
				"error": "User not found",
			}, "User not found")
		}

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
				return http.ServerError(c, err, "Failed to reactivate project member")
			}

			existingMember.Role = &request.Role
			existingMember.UpdatedAt = now
			existingMember.DeletedAt = nil

			slog.Info("project.member_added", "project_id", projectID.Hex(), "member_id", userIDToAdd.Hex(), "user_id", userID.Hex())

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
			return http.ServerError(c, err, "Member not added")
		}

		if insertedID, ok := result.InsertedID.(bson.ObjectID); ok {
			member.ID = &insertedID
		}

		slog.Info("project.member_added", "project_id", projectID.Hex(), "member_id", userIDToAdd.Hex(), "user_id", userID.Hex())

		return http.Success(c, 201, member)
	}
}

// @Summary		Muda a role de um membro do projeto
// @Description	Só admin promove, rebaixa ou mexe em admin.
// @Tags			project-members
// @Accept			json
// @Produce		json
// @Param			id		path		string				true	"ID do projeto (ObjectID)"
// @Param			body	body		dto.MemberRequest	true	"Usuário e nova role"
// @Success		200		{object}	dto.MessageEnvelope
// @Failure		400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure		401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure		403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure		404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Failure		409		{object}	dto.ErrorResponse	"O projeto precisa de pelo menos um admin"
// @Security		BearerAuth
// @Router			/project/member/{id} [patch]
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from auth",
			}, "Fail to get user from auth")
		}

		userRole, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionProjectManageMembers)

		if err != nil {
			return respondAuthError(c, err)
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

		var target models.ProjectMember

		err = collection.FindOne(ctx, bson.M{
			"project_id": projectID,
			"user_id":    subjectUserID,
			"$or":        services.NotDeleted(),
		}).Decode(&target)

		if err != nil || target.Role == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project member not found",
			}, "Project member not found")
		}

		if !services.CanChangeMemberRole(userRole, *target.Role, request.Role) {
			return http.Error(c, 403, fiber.Map{
				"error": "Only admins can manage admins",
			}, "Only admins can manage admins")
		}

		if request.Role != services.RoleAdmin {
			if err := services.EnsureNotLastAdmin(ctx, "project_members", "project_id", projectID, *target.Role); err != nil {
				return respondLastAdminError(c, err, "Project must have at least one admin")
			}
		}

		result := collection.FindOneAndUpdate(
			ctx,
			bson.M{
				"_id": target.ID,
			},
			bson.M{
				"$set": bson.M{
					"role":       request.Role,
					"updated_at": time.Now(),
				},
			},
		)

		if result.Err() != nil {
			return http.ServerError(c, result.Err(), "Error to update project member")
		}

		return http.Success(c, 200, fiber.Map{
			"message": "Member updated",
		})
	}
}

// @Summary	Remove um membro do projeto
// @Tags		project-members
// @Accept		json
// @Produce	json
// @Param		id		path		string					true	"ID do projeto (ObjectID)"
// @Param		body	body		dto.RemoveMemberRequest	true	"Usuário a remover"
// @Success	200		{object}	dto.MessageEnvelope
// @Failure	400		{object}	dto.ErrorResponse	"Parâmetro ou corpo inválido"
// @Failure	401		{object}	dto.ErrorResponse	"Sem token ou token inválido"
// @Failure	403		{object}	dto.ErrorResponse	"Sem permissão para a ação"
// @Failure	404		{object}	dto.ErrorResponse	"Não existe ou o usuário não é membro"
// @Failure	409		{object}	dto.ErrorResponse	"O projeto precisa de pelo menos um admin"
// @Security	BearerAuth
// @Router		/project/member/{id} [delete]
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

		userID, err := middleware.CurrentUserID(c)

		if err != nil {
			return http.Error(c, 403, fiber.Map{
				"error": "Fail to get user from auth",
			}, "Fail to get user from auth")
		}

		userRole, err := services.AuthorizeProject(context.Background(), userID, projectID, services.ActionProjectManageMembers)

		if err != nil {
			return respondAuthError(c, err)
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

		var target models.ProjectMember

		err = collection.FindOne(ctx, bson.M{
			"project_id": projectID,
			"user_id":    subjectUserID,
			"$or":        services.NotDeleted(),
		}).Decode(&target)

		if err != nil || target.Role == nil {
			return http.Error(c, 404, fiber.Map{
				"error": "Project member not found",
			}, "Project member not found")
		}

		if !services.CanChangeMemberRole(userRole, *target.Role, "") {
			return http.Error(c, 403, fiber.Map{
				"error": "Only admins can manage admins",
			}, "Only admins can manage admins")
		}

		if err := services.EnsureNotLastAdmin(ctx, "project_members", "project_id", projectID, *target.Role); err != nil {
			return respondLastAdminError(c, err, "Project must have at least one admin")
		}

		now := time.Now()

		result, err := collection.UpdateOne(
			ctx,
			bson.M{
				"project_id": &projectID,
				"user_id":    subjectUserID,
				"$or":        services.NotDeleted(),
			},
			bson.M{
				"$set": bson.M{
					"deleted_at": now,
					"updated_at": now,
				},
			},
		)

		if err != nil {
			return http.ServerError(c, err, "Member not deleted")
		}

		if result.MatchedCount == 0 {
			return http.Error(c, 404, fiber.Map{
				"error": "Project member not found",
			}, "Project member not found")
		}

		slog.Info("project.member_removed", "project_id", projectID.Hex(), "member_id", subjectUserID.Hex(), "user_id", userID.Hex())

		return http.Success(c, 200, fiber.Map{
			"message": "Member deleted",
		})
	}
}
