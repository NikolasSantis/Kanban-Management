package models

import (
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// MinDueDateYear: datas antes disso são recusadas.
	MinDueDateYear = 2000
	// MaxDueDateYearsAhead: datas mais distantes que isso no futuro são recusadas.
	MaxDueDateYearsAhead = 10
)

// ErrInvalidDate: texto que não é RFC 3339 nem YYYY-MM-DD.
var ErrInvalidDate = errors.New("invalid date")

// ErrDueDateOutOfRange: data antes de 2000 ou a mais de 10 anos no futuro.
var ErrDueDateOutOfRange = errors.New("due date out of range")

const InvalidDueDateMessage = "Invalid due_date. Use RFC 3339 (2026-12-01T18:00:00Z) or YYYY-MM-DD, between 2000 and 10 years ahead"

// ParseDate aceita RFC 3339 (qualquer fuso) ou só a data (YYYY-MM-DD, que vira
// o fim do dia, 23:59:59, em UTC). O resultado é sempre UTC. Sem limites.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)

	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC(), nil
	}

	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC), nil
	}

	return time.Time{}, ErrInvalidDate
}

// ParseDueDateAt é ParseDueDate com o "agora" injetável (para testes).
func ParseDueDateAt(s string, now time.Time) (time.Time, error) {
	t, err := ParseDate(s)

	if err != nil {
		return time.Time{}, err
	}

	if t.Year() < MinDueDateYear || t.After(now.UTC().AddDate(MaxDueDateYearsAhead, 0, 0)) {
		return time.Time{}, ErrDueDateOutOfRange
	}

	return t, nil
}

// ParseDueDate valida a data de entrega usando o relógio atual.
func ParseDueDate(s string) (time.Time, error) {
	return ParseDueDateAt(s, time.Now())
}

// CardResponse é o card como a API devolve: com is_overdue calculado.
type CardResponse struct {
	Card
	IsOverdue bool `json:"is_overdue"`
}

// ToCardResponse calcula is_overdue = tem data, não está concluído e a data já passou.
func ToCardResponse(card Card, now time.Time) CardResponse {
	// Cards antigos podem não ter essas listas: a API sempre devolve [] (nunca null).
	if card.LabelIDs == nil {
		card.LabelIDs = []bson.ObjectID{}
	}

	if card.AssigneeIDs == nil {
		card.AssigneeIDs = []bson.ObjectID{}
	}

	return CardResponse{
		Card:      card,
		IsOverdue: card.DueDate != nil && card.CompletedAt == nil && card.DueDate.Before(now),
	}
}

// ToCardResponses converte uma lista, devolvendo [] (nunca nil).
func ToCardResponses(cards []Card, now time.Time) []CardResponse {
	responses := make([]CardResponse, 0, len(cards))

	for _, card := range cards {
		responses = append(responses, ToCardResponse(card, now))
	}

	return responses
}
