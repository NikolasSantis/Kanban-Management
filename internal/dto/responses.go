package dto

import "time"

// ErrorData é o "data" das respostas de erro.
type ErrorData struct {
	Error string `json:"error" example:"Not found"`
}

// ErrorResponse é o envelope de erro: {success: false, data: {error}, error}.
type ErrorResponse struct {
	Success bool      `json:"success" example:"false"`
	Data    ErrorData `json:"data"`
	Error   string    `json:"error" example:"Not found"`
}

// ColumnsPage é o "data" de GET /card/board/:board_id sem filtros: a página de
// cada coluna ativa do board, indexada pelo id da coluna.
type ColumnsPage struct {
	Columns map[string]PageCards `json:"columns"`
}

// TokenData é o "data" do login.
type TokenData struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// MessageData é o "data" de respostas simples ({"message": "..."}).
type MessageData struct {
	Message string `json:"message" example:"Updated"`
}

// HealthData é o "data" de GET /health.
type HealthData struct {
	Status string `json:"status" enums:"ok,unavailable" example:"ok"`
}

// UserResponse é o usuário SEM senha (nunca use models.User em respostas).
type UserResponse struct {
	ID              string    `json:"id" example:"6a8f82d0a830f44ec741fb7c"`
	Name            string    `json:"name" example:"Ana Souza"`
	Email           string    `json:"email" example:"ana@exemplo.com"`
	EmailVerifiedAt time.Time `json:"email_verified_at"`
	ExpirationDate  time.Time `json:"expiration_date"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RegisterData é o "data" do cadastro.
type RegisterData struct {
	Success bool `json:"success" example:"true"`
}

// UpdateResultData é o resultado cru do update do Mongo devolvido por PATCH /users/{id}.
type UpdateResultData struct {
	Acknowledged  bool  `json:"Acknowledged" example:"true"`
	MatchedCount  int64 `json:"MatchedCount" example:"1"`
	ModifiedCount int64 `json:"ModifiedCount" example:"1"`
	UpsertedCount int64 `json:"UpsertedCount" example:"0"`
}

// HealthErrorResponse é a resposta 503 de GET /health.
type HealthErrorResponse struct {
	Success bool       `json:"success" example:"false"`
	Data    HealthData `json:"data"`
	Error   string     `json:"error" example:"Database unavailable"`
}
