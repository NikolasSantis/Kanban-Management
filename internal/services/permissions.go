package services

import (
	"context"
	"errors"
	"slices"

	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Action string

const (
	ActionProjectView          Action = "project:view"
	ActionProjectUpdate        Action = "project:update"
	ActionProjectDelete        Action = "project:delete"
	ActionProjectManageMembers Action = "project:manage_members"
	ActionBoardView            Action = "board:view"
	ActionBoardManage          Action = "board:manage" // criar/editar/deletar board e colunas
	ActionCardView             Action = "card:view"
	ActionCardCreate           Action = "card:create"
	ActionCardUpdate           Action = "card:update" // editar/mover qualquer card
	ActionCardDelete           Action = "card:delete"
	ActionLabelManage          Action = "label:manage"
	ActionCommentCreate        Action = "comment:create"
	ActionCommentModerate      Action = "comment:moderate" // editar/apagar comentário de OUTRA pessoa
	ActionAttachmentCreate     Action = "attachment:create"
	ActionAttachmentModerate   Action = "attachment:moderate"

	ActionWorkspaceView          Action = "workspace:view"
	ActionWorkspaceUpdate        Action = "workspace:update"
	ActionWorkspaceDelete        Action = "workspace:delete"
	ActionWorkspaceManageMembers Action = "workspace:manage_members"
)

const (
	RoleMember  = "member"
	RoleManager = "manager"
	RoleAdmin   = "admin"
)

var (
	// ErrNotMember: o usuário não é membro (ou o recurso não existe). Responder 404.
	ErrNotMember = errors.New("user is not a member")
	// ErrForbidden: é membro, mas a role não permite a ação. Responder 403.
	ErrForbidden = errors.New("action not allowed for this role")
)

var memberPermissions = []Action{
	ActionProjectView,
	ActionBoardView,
	ActionCardView,
	ActionCardCreate,
	ActionCardUpdate,
	ActionCommentCreate,
	ActionAttachmentCreate,
	ActionWorkspaceView,
}

// manager = member + gerenciar boards, labels, moderação, editar projeto e membros.
var managerPermissions = slices.Concat(memberPermissions, []Action{
	ActionProjectUpdate,
	ActionProjectManageMembers,
	ActionBoardManage,
	ActionCardDelete,
	ActionLabelManage,
	ActionCommentModerate,
	ActionAttachmentModerate,
	ActionWorkspaceUpdate,
	ActionWorkspaceManageMembers,
})

// admin = manager + deletar projeto/workspace.
var adminPermissions = slices.Concat(managerPermissions, []Action{
	ActionProjectDelete,
	ActionWorkspaceDelete,
})

var rolePermissions = map[string][]Action{
	RoleMember:  memberPermissions,
	RoleManager: managerPermissions,
	RoleAdmin:   adminPermissions,
}

// Can informa se a role pode executar a ação. Role desconhecida nunca pode.
func Can(role string, action Action) bool {
	return slices.Contains(rolePermissions[role], action)
}

// AuthorizeProject retorna a role do usuário no projeto ou ErrNotMember /
// ErrForbidden. Projeto inexistente ou deletado também retorna ErrNotMember.
func AuthorizeProject(ctx context.Context, userID, projectID bson.ObjectID, action Action) (string, error) {
	count, err := database.GetCollection("projects").CountDocuments(ctx, bson.M{
		"_id": projectID,
		"$or": NotDeleted(),
	})

	if err != nil || count == 0 {
		return "", ErrNotMember
	}

	role, err := GetProjectUserRole(userID, projectID)

	if err != nil {
		return "", ErrNotMember
	}

	if !Can(role, action) {
		return role, ErrForbidden
	}

	return role, nil
}

// AuthorizeWorkspace é a versão de AuthorizeProject para workspaces.
func AuthorizeWorkspace(ctx context.Context, userID, workspaceID bson.ObjectID, action Action) (string, error) {
	if !WorkspaceExists(workspaceID) {
		return "", ErrNotMember
	}

	role, err := GetWorkspaceUserRole(userID, workspaceID)

	if err != nil {
		return "", ErrNotMember
	}

	if !Can(role, action) {
		return role, ErrForbidden
	}

	return role, nil
}
