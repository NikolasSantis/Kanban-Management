package dto

import (
	"kanban-management/internal/models"
	"kanban-management/internal/pagination"
)

// Tipos concretos das páginas ({items, pagination}). O swag não resolve genéricos
// aninhados (SuccessResponse[Page[T]]), então cada listagem tem o seu tipo.

type PageWorkspaces struct {
	Items      []models.Workspace `json:"items"`
	Pagination pagination.Meta    `json:"pagination"`
}

type PageWorkspaceMembers struct {
	Items      []models.WorkspaceMember `json:"items"`
	Pagination pagination.Meta          `json:"pagination"`
}

type PageProjects struct {
	Items      []models.Project `json:"items"`
	Pagination pagination.Meta  `json:"pagination"`
}

type PageProjectMembers struct {
	Items      []models.ProjectMember `json:"items"`
	Pagination pagination.Meta        `json:"pagination"`
}

type PageBoards struct {
	Items      []models.Board  `json:"items"`
	Pagination pagination.Meta `json:"pagination"`
}

type PageCards struct {
	Items      []models.CardResponse `json:"items"`
	Pagination pagination.Meta       `json:"pagination"`
}

type PageLabels struct {
	Items      []models.Label  `json:"items"`
	Pagination pagination.Meta `json:"pagination"`
}

type PageComments struct {
	Items      []models.CommentResponse `json:"items"`
	Pagination pagination.Meta          `json:"pagination"`
}

type PageAttachments struct {
	Items      []models.AttachmentResponse `json:"items"`
	Pagination pagination.Meta             `json:"pagination"`
}
