// Package dto reúne os tipos usados SÓ para documentar a API no Swagger
// (corpos de request e formatos de resposta). Não têm lógica: os handlers
// continuam lendo o corpo do jeito que já faziam.
package dto

// ---------- auth e usuários ----------

type LoginRequest struct {
	Email    string `json:"email" example:"ana@exemplo.com"`
	Password string `json:"password" example:"minha-senha-forte"`
}

type RegisterRequest struct {
	Name     string `json:"name" example:"Ana Souza"`
	Email    string `json:"email" example:"ana@exemplo.com"`
	Password string `json:"password" example:"minha-senha-forte"`
}

// UpdateUserRequest: todos os campos são opcionais; só o próprio usuário edita a conta.
type UpdateUserRequest struct {
	Name  *string `json:"name,omitempty" example:"Ana Souza"`
	Email *string `json:"email,omitempty" example:"ana@exemplo.com"`
}

// ---------- workspaces e projetos ----------

type CreateWorkspaceRequest struct {
	Name        string `json:"name" example:"Minha empresa"`
	Description string `json:"description" example:"Tudo da minha empresa"`
}

type UpdateWorkspaceRequest struct {
	Name        *string `json:"name,omitempty" example:"Minha empresa"`
	Description *string `json:"description,omitempty" example:"Tudo da minha empresa"`
}

type CreateProjectRequest struct {
	WorkspaceID string `json:"workspace_id" example:"6a8f82d0a830f44ec741fb7c"`
	Name        string `json:"name" example:"Aplicativo móvel"`
	Description string `json:"description" example:"Versão para celular"`
}

type UpdateProjectRequest struct {
	Name        *string `json:"name,omitempty" example:"Aplicativo móvel"`
	Description *string `json:"description,omitempty" example:"Versão para celular"`
	Status      *string `json:"status,omitempty" example:"active"`
}

// MemberRequest adiciona ou muda a role de um membro (workspace ou projeto).
type MemberRequest struct {
	UserID string `json:"user_id" example:"6a8f82d0a830f44ec741fb7c"`
	Role   string `json:"role" enums:"member,manager,admin" example:"member"`
}

// RemoveMemberRequest remove um membro (workspace ou projeto).
type RemoveMemberRequest struct {
	UserID string `json:"user_id" example:"6a8f82d0a830f44ec741fb7c"`
}

// ---------- boards e colunas ----------

type ColumnInput struct {
	ID       string  `json:"id,omitempty" example:"todo"`
	Name     string  `json:"name" example:"A fazer"`
	Position int32   `json:"position,omitempty" example:"1"`
	Color    *string `json:"color,omitempty" example:"#0079bf"`
	Icon     *string `json:"icon,omitempty" example:"list"`
}

type CreateBoardRequest struct {
	Name        string        `json:"name" example:"Sprint 12"`
	Description *string       `json:"description,omitempty" example:"Tarefas da sprint"`
	Color       *string       `json:"color,omitempty" example:"#0079bf"`
	Background  *string       `json:"background,omitempty" example:"blue"`
	Columns     []ColumnInput `json:"columns,omitempty"`
}

// UpdateBoardRequest: todos os campos são opcionais. Campos internos (project_id,
// created_at, search_text...) são ignorados.
type UpdateBoardRequest struct {
	Name        *string       `json:"name,omitempty" example:"Sprint 12"`
	Description *string       `json:"description,omitempty" example:"Tarefas da sprint"`
	Color       *string       `json:"color,omitempty" example:"#0079bf"`
	Background  *string       `json:"background,omitempty" example:"blue"`
	Columns     []ColumnInput `json:"columns,omitempty"`
}

type CreateColumnRequest struct {
	ID       *string `json:"id,omitempty" example:"review"`
	Name     string  `json:"name" example:"Em revisão"`
	Color    *string `json:"color,omitempty" example:"#f2d600"`
	Icon     *string `json:"icon,omitempty" example:"eye"`
	Position *int32  `json:"position,omitempty" example:"3"`
}

type UpdateColumnRequest struct {
	Name     *string `json:"name,omitempty" example:"Em revisão"`
	Color    *string `json:"color,omitempty" example:"#f2d600"`
	Icon     *string `json:"icon,omitempty" example:"eye"`
	Position *int32  `json:"position,omitempty" example:"3"`
}

// ---------- cards ----------

type CreateCardRequest struct {
	Title       string   `json:"title" example:"Implementar login"`
	Description *string  `json:"description,omitempty" example:"Usar JWT"`
	ColumnID    string   `json:"column_id" example:"todo"`
	AssigneeIDs []string `json:"assignee_ids,omitempty" example:"6a8f82d0a830f44ec741fb7c"`
	Priority    *string  `json:"priority,omitempty" enums:"low,medium,high,urgent" example:"high"`
	// DueDate: RFC 3339 ou só a data (YYYY-MM-DD, vira 23:59:59Z), entre 2000 e 10 anos à frente.
	DueDate  *string  `json:"due_date,omitempty" example:"2026-12-01T23:59:59Z"`
	LabelIDs []string `json:"label_ids,omitempty" example:"6a8f82d0a830f44ec741fb7c"`
}

// UpdateCardRequest: todos os campos são opcionais. due_date: string define, null remove.
type UpdateCardRequest struct {
	Title       *string   `json:"title,omitempty" example:"Implementar login"`
	Description *string   `json:"description,omitempty" example:"Usar JWT"`
	AssigneeIDs *[]string `json:"assignee_ids,omitempty" example:"6a8f82d0a830f44ec741fb7c"`
	Priority    *string   `json:"priority,omitempty" enums:"low,medium,high,urgent" example:"urgent"`
	DueDate     *string   `json:"due_date,omitempty" example:"2026-12-01"`
	LabelIDs    *[]string `json:"label_ids,omitempty" example:"6a8f82d0a830f44ec741fb7c"`
}

// MoveCardRequest: before_card_id é o card imediatamente acima da nova posição e
// after_card_id o imediatamente abaixo (ambos na coluna de destino). Sem nenhum,
// o card vai para o fim da coluna.
type MoveCardRequest struct {
	ColumnID     string  `json:"column_id" example:"doing"`
	BeforeCardID *string `json:"before_card_id,omitempty" example:"6a8f82d0a830f44ec741fb7c"`
	AfterCardID  *string `json:"after_card_id,omitempty" example:"6a8f82d0a830f44ec741fb7d"`
}

// ---------- labels, comentários e anexos ----------

type CreateLabelRequest struct {
	Name  string  `json:"name" example:"Bug"`
	Color *string `json:"color,omitempty" example:"#eb5a46"`
}

type UpdateLabelRequest struct {
	Name  *string `json:"name,omitempty" example:"Bug"`
	Color *string `json:"color,omitempty" example:"#eb5a46"`
}

type CreateCommentRequest struct {
	Content string `json:"content" example:"Vou olhar isso amanhã."`
}

type UpdateCommentRequest struct {
	Content string `json:"content" example:"Vou olhar isso hoje."`
}

type CreateLinkAttachmentRequest struct {
	Name string `json:"name" example:"Especificação"`
	URL  string `json:"url" example:"https://exemplo.com/spec"`
}
