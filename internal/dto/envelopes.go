package dto

import "kanban-management/internal/models"

// Envelopes {success, data, error} concretos, um por tipo de "data". Existem só para
// o Swagger: o swag não resolveu um envelope genérico (SuccessResponse[T]) aqui.

// StringEnvelope: {success, data: string, error}.
type StringEnvelope struct {
	Success bool    `json:"success" example:"true"`
	Data    string  `json:"data"`
	Error   *string `json:"error"`
}

// MessageEnvelope: {success, data: MessageData, error}.
type MessageEnvelope struct {
	Success bool        `json:"success" example:"true"`
	Data    MessageData `json:"data"`
	Error   *string     `json:"error"`
}

// TokenEnvelope: {success, data: TokenData, error}.
type TokenEnvelope struct {
	Success bool      `json:"success" example:"true"`
	Data    TokenData `json:"data"`
	Error   *string   `json:"error"`
}

// RegisterEnvelope: {success, data: RegisterData, error}.
type RegisterEnvelope struct {
	Success bool         `json:"success" example:"true"`
	Data    RegisterData `json:"data"`
	Error   *string      `json:"error"`
}

// UserEnvelope: {success, data: UserResponse, error}.
type UserEnvelope struct {
	Success bool         `json:"success" example:"true"`
	Data    UserResponse `json:"data"`
	Error   *string      `json:"error"`
}

// UpdateResultEnvelope: {success, data: UpdateResultData, error}.
type UpdateResultEnvelope struct {
	Success bool             `json:"success" example:"true"`
	Data    UpdateResultData `json:"data"`
	Error   *string          `json:"error"`
}

// HealthEnvelope: {success, data: HealthData, error}.
type HealthEnvelope struct {
	Success bool       `json:"success" example:"true"`
	Data    HealthData `json:"data"`
	Error   *string    `json:"error"`
}

// WorkspaceEnvelope: {success, data: models.Workspace, error}.
type WorkspaceEnvelope struct {
	Success bool             `json:"success" example:"true"`
	Data    models.Workspace `json:"data"`
	Error   *string          `json:"error"`
}

// WorkspaceMemberEnvelope: {success, data: models.WorkspaceMember, error}.
type WorkspaceMemberEnvelope struct {
	Success bool                   `json:"success" example:"true"`
	Data    models.WorkspaceMember `json:"data"`
	Error   *string                `json:"error"`
}

// ProjectEnvelope: {success, data: models.Project, error}.
type ProjectEnvelope struct {
	Success bool           `json:"success" example:"true"`
	Data    models.Project `json:"data"`
	Error   *string        `json:"error"`
}

// ProjectMemberEnvelope: {success, data: models.ProjectMember, error}.
type ProjectMemberEnvelope struct {
	Success bool                 `json:"success" example:"true"`
	Data    models.ProjectMember `json:"data"`
	Error   *string              `json:"error"`
}

// BoardEnvelope: {success, data: models.Board, error}.
type BoardEnvelope struct {
	Success bool         `json:"success" example:"true"`
	Data    models.Board `json:"data"`
	Error   *string      `json:"error"`
}

// ColumnEnvelope: {success, data: models.Column, error}.
type ColumnEnvelope struct {
	Success bool          `json:"success" example:"true"`
	Data    models.Column `json:"data"`
	Error   *string       `json:"error"`
}

// CardEnvelope: {success, data: models.CardResponse, error}.
type CardEnvelope struct {
	Success bool                `json:"success" example:"true"`
	Data    models.CardResponse `json:"data"`
	Error   *string             `json:"error"`
}

// CardListEnvelope: {success, data: []models.CardResponse, error}.
type CardListEnvelope struct {
	Success bool                  `json:"success" example:"true"`
	Data    []models.CardResponse `json:"data"`
	Error   *string               `json:"error"`
}

// LabelEnvelope: {success, data: models.Label, error}.
type LabelEnvelope struct {
	Success bool         `json:"success" example:"true"`
	Data    models.Label `json:"data"`
	Error   *string      `json:"error"`
}

// CommentEnvelope: {success, data: models.CommentResponse, error}.
type CommentEnvelope struct {
	Success bool                   `json:"success" example:"true"`
	Data    models.CommentResponse `json:"data"`
	Error   *string                `json:"error"`
}

// AttachmentEnvelope: {success, data: models.AttachmentResponse, error}.
type AttachmentEnvelope struct {
	Success bool                      `json:"success" example:"true"`
	Data    models.AttachmentResponse `json:"data"`
	Error   *string                   `json:"error"`
}

// PageWorkspacesEnvelope: {success, data: PageWorkspaces, error}.
type PageWorkspacesEnvelope struct {
	Success bool           `json:"success" example:"true"`
	Data    PageWorkspaces `json:"data"`
	Error   *string        `json:"error"`
}

// PageWorkspaceMembersEnvelope: {success, data: PageWorkspaceMembers, error}.
type PageWorkspaceMembersEnvelope struct {
	Success bool                 `json:"success" example:"true"`
	Data    PageWorkspaceMembers `json:"data"`
	Error   *string              `json:"error"`
}

// PageProjectsEnvelope: {success, data: PageProjects, error}.
type PageProjectsEnvelope struct {
	Success bool         `json:"success" example:"true"`
	Data    PageProjects `json:"data"`
	Error   *string      `json:"error"`
}

// PageProjectMembersEnvelope: {success, data: PageProjectMembers, error}.
type PageProjectMembersEnvelope struct {
	Success bool               `json:"success" example:"true"`
	Data    PageProjectMembers `json:"data"`
	Error   *string            `json:"error"`
}

// PageBoardsEnvelope: {success, data: PageBoards, error}.
type PageBoardsEnvelope struct {
	Success bool       `json:"success" example:"true"`
	Data    PageBoards `json:"data"`
	Error   *string    `json:"error"`
}

// PageCardsEnvelope: {success, data: PageCards, error}.
type PageCardsEnvelope struct {
	Success bool      `json:"success" example:"true"`
	Data    PageCards `json:"data"`
	Error   *string   `json:"error"`
}

// PageLabelsEnvelope: {success, data: PageLabels, error}.
type PageLabelsEnvelope struct {
	Success bool       `json:"success" example:"true"`
	Data    PageLabels `json:"data"`
	Error   *string    `json:"error"`
}

// PageCommentsEnvelope: {success, data: PageComments, error}.
type PageCommentsEnvelope struct {
	Success bool         `json:"success" example:"true"`
	Data    PageComments `json:"data"`
	Error   *string      `json:"error"`
}

// PageAttachmentsEnvelope: {success, data: PageAttachments, error}.
type PageAttachmentsEnvelope struct {
	Success bool            `json:"success" example:"true"`
	Data    PageAttachments `json:"data"`
	Error   *string         `json:"error"`
}

// ColumnsPageEnvelope: {success, data: ColumnsPage, error}.
type ColumnsPageEnvelope struct {
	Success bool        `json:"success" example:"true"`
	Data    ColumnsPage `json:"data"`
	Error   *string     `json:"error"`
}
