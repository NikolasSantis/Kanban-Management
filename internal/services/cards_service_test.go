package services_test

import (
	"strings"
	"testing"
	"time"

	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T { return &v }

func TestPositionBetween(t *testing.T) {
	cases := []struct {
		name          string
		before, after *float64
		want          float64
	}{
		{"coluna vazia", nil, nil, 1000},
		{"só antes: vai para o fim", ptr(3000.0), nil, 4000},
		{"só depois: vai para o início", nil, ptr(1000.0), 500},
		{"entre dois cards", ptr(1000.0), ptr(2000.0), 1500},
		{"entre dois cards próximos", ptr(1000.0), ptr(1001.0), 1000.5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.InDelta(t, tc.want, services.PositionBetween(tc.before, tc.after), 1e-9)
		})
	}
}

func TestColumnExists(t *testing.T) {
	deletedAt := time.Now()
	board := &models.Board{Columns: []models.Column{
		{ID: ptr("todo")},
		{ID: ptr("done"), DeletedAt: &deletedAt},
		{ID: nil},
	}}

	assert.True(t, services.ColumnExists(board, "todo"))
	assert.False(t, services.ColumnExists(board, "done"), "coluna deletada não conta")
	assert.False(t, services.ColumnExists(board, "inexistente"))
	assert.False(t, services.ColumnExists(board, ""))
	assert.False(t, services.ColumnExists(nil, "todo"))
}

func TestNormalizeCardTitle(t *testing.T) {
	cases := []struct {
		name    string
		title   string
		want    string
		wantErr bool
	}{
		{"normal", "Fazer deploy", "Fazer deploy", false},
		{"remove espaços nas pontas", "  Título  ", "Título", false},
		{"vazio", "", "", true},
		{"só espaços", "   ", "", true},
		{"200 caracteres", strings.Repeat("a", 200), strings.Repeat("a", 200), false},
		{"201 caracteres", strings.Repeat("a", 201), "", true},
		{"200 runes multibyte", strings.Repeat("ã", 200), strings.Repeat("ã", 200), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := services.NormalizeCardTitle(tc.title)

			if tc.wantErr {
				assert.ErrorIs(t, err, services.ErrInvalidCard)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateCardDescription(t *testing.T) {
	assert.NoError(t, services.ValidateCardDescription(""))
	assert.NoError(t, services.ValidateCardDescription(strings.Repeat("a", 5000)))
	assert.ErrorIs(t, services.ValidateCardDescription(strings.Repeat("a", 5001)), services.ErrInvalidCard)
}
