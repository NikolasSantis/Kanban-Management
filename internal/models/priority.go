package models

import (
	"errors"
	"strings"
)

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// PriorityDefault é a prioridade usada quando o cliente não envia nenhuma.
const PriorityDefault = PriorityMedium

// ErrInvalidPriority: valor fora de low, medium, high, urgent.
var ErrInvalidPriority = errors.New("invalid priority")

// InvalidPriorityMessage é a mensagem de 400 devolvida pela API.
const InvalidPriorityMessage = "Invalid priority. Allowed: low, medium, high, urgent"

var priorityLevels = map[Priority]int{
	PriorityLow:    1,
	PriorityMedium: 2,
	PriorityHigh:   3,
	PriorityUrgent: 4,
}

// ParsePriority aceita maiúsculas/minúsculas e espaços nas pontas.
func ParsePriority(s string) (Priority, error) {
	p := Priority(strings.ToLower(strings.TrimSpace(s)))

	if _, ok := priorityLevels[p]; !ok {
		return "", ErrInvalidPriority
	}

	return p, nil
}

// Level devolve o nível numérico usado para ordenar no Mongo (low=1 ... urgent=4).
// Prioridade desconhecida devolve 0.
func (p Priority) Level() int {
	return priorityLevels[p]
}
