package integration_test

import (
	"strings"
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s cardScenario) columnsPath() string { return "/board/" + s.boardID + "/columns" }

// column devolve a coluna do board pelo id (nil se não existir).
func (s cardScenario) column(t *testing.T, columnID string) map[string]any {
	t.Helper()

	status, env := s.do(t, "GET", "/board/"+s.boardID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	columns, _ := env.Map()["columns"].([]any)
	for _, raw := range columns {
		column := raw.(map[string]any)
		if column["id"] == columnID {
			return column
		}
	}

	return nil
}

func TestAddColumn(t *testing.T) {
	s := newCardScenario(t)

	t.Run("admin adiciona com id gerado e position seguinte", func(t *testing.T) {
		status, env := s.do(t, "POST", s.columnsPath(), s.admin.Token, map[string]any{"name": "  Review  ", "color": "#f00", "icon": "eye"})

		require.Equal(t, 201, status)
		column := env.Map()
		id, _ := column["id"].(string)
		assert.Len(t, id, 24, "id gerado com bson.NewObjectID().Hex()")
		assert.Equal(t, "Review", column["name"])
		assert.Equal(t, float64(3), column["position"], "as colunas todo e doing têm position 1 e 2")

		stored := s.column(t, id)
		require.NotNil(t, stored)
		assert.Equal(t, "#f00", stored["color"])
	})

	t.Run("manager adiciona com id escolhido pelo cliente", func(t *testing.T) {
		status, env := s.do(t, "POST", s.columnsPath(), s.manager.Token, map[string]any{"id": "done", "name": "Done", "position": 10})

		require.Equal(t, 201, status)
		assert.Equal(t, "done", env.Map()["id"])
		assert.Equal(t, float64(10), env.Map()["position"])
	})

	t.Run("a coluna nova aceita cards", func(t *testing.T) {
		status, _ := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": "x", "column_id": "done"})

		assert.Equal(t, 201, status)
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"id repetido", s.columnsPath(), s.admin.Token, map[string]any{"id": "todo", "name": "x"}, 409},
		{"sem nome", s.columnsPath(), s.admin.Token, map[string]any{"color": "#fff"}, 400},
		{"nome em branco", s.columnsPath(), s.admin.Token, map[string]any{"name": "  "}, 400},
		{"nome com 101 caracteres", s.columnsPath(), s.admin.Token, map[string]any{"name": strings.Repeat("a", 101)}, 400},
		{"id em branco", s.columnsPath(), s.admin.Token, map[string]any{"id": " ", "name": "x"}, 400},
		{"corpo inválido", s.columnsPath(), s.admin.Token, "texto", 400},
		{"member não adiciona", s.columnsPath(), s.member.Token, map[string]any{"name": "x"}, 403},
		{"não-membro recebe 404", s.columnsPath(), s.outsider.Token, map[string]any{"name": "x"}, 404},
		{"sem autenticação", s.columnsPath(), "", map[string]any{"name": "x"}, 401},
		{"id de board inválido", "/board/xyz/columns", s.admin.Token, map[string]any{"name": "x"}, 400},
		{"board inexistente", "/board/000000000000000000000001/columns", s.admin.Token, map[string]any{"name": "x"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestPatchColumn(t *testing.T) {
	s := newCardScenario(t)
	path := s.columnsPath() + "/todo"

	t.Run("edita nome, cor, ícone e position sem mexer nas outras colunas", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{
			"name": "A fazer", "color": "#0f0", "icon": "list", "position": 5,
		})
		require.Equal(t, 200, status)

		todo := s.column(t, "todo")
		assert.Equal(t, "A fazer", todo["name"])
		assert.Equal(t, "#0f0", todo["color"])
		assert.Equal(t, "list", todo["icon"])
		assert.Equal(t, float64(5), todo["position"])

		doing := s.column(t, "doing")
		assert.Equal(t, "doing", doing["name"], "a outra coluna não pode ter mudado")
		assert.Equal(t, float64(2), doing["position"])
	})

	t.Run("id da coluna não pode ser alterado", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"id": "outro-id", "name": "Mesmo assim"})
		require.Equal(t, 200, status)

		assert.NotNil(t, s.column(t, "todo"))
		assert.Nil(t, s.column(t, "outro-id"))
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"só campos fora da allowlist", path, s.admin.Token, map[string]any{"id": "x", "deleted_at": "2020-01-01T00:00:00Z"}, 400},
		{"nome em branco", path, s.admin.Token, map[string]any{"name": " "}, 400},
		{"JSON inválido", path, s.admin.Token, "texto", 400},
		{"coluna inexistente", s.columnsPath() + "/nao-existe", s.admin.Token, map[string]any{"name": "x"}, 404},
		{"member não edita", path, s.member.Token, map[string]any{"name": "x"}, 403},
		{"não-membro recebe 404", path, s.outsider.Token, map[string]any{"name": "x"}, 404},
		{"id de board inválido", "/board/xyz/columns/todo", s.admin.Token, map[string]any{"name": "x"}, 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "PATCH", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestDeleteColumn(t *testing.T) {
	s := newCardScenario(t)
	path := s.columnsPath() + "/todo"

	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "um")
	deletedCardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "apagado")

	status, _ := s.do(t, "DELETE", "/card/"+deletedCardID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	t.Run("member não deleta", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", path, s.member.Token, nil)
		assert.Equal(t, 403, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", path, s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("coluna inexistente", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", s.columnsPath()+"/nao-existe", s.admin.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("com card ativo devolve 409 (card deletado não conta)", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", path, s.admin.Token, nil)

		assert.Equal(t, 409, status)
		assert.Nil(t, s.column(t, "todo")["deleted_at"], "a coluna não pode ter sido deletada")
	})

	t.Run("depois de mover os cards, deleta (soft delete)", func(t *testing.T) {
		status, _ := s.move(t, s.admin.Token, cardID, map[string]any{"column_id": "doing"})
		require.Equal(t, 200, status)

		status, _ = s.do(t, "DELETE", path, s.admin.Token, nil)
		require.Equal(t, 200, status)

		todo := s.column(t, "todo")
		require.NotNil(t, todo, "a coluna continua no documento")
		assert.NotNil(t, todo["deleted_at"])
		assert.Nil(t, s.column(t, "doing")["deleted_at"], "a outra coluna não pode ter sido afetada")
	})

	t.Run("coluna deletada não aceita cards, nem mudanças, nem novo delete", func(t *testing.T) {
		status, _ := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": "x", "column_id": "todo"})
		assert.Equal(t, 400, status)

		status, _ = s.move(t, s.admin.Token, cardID, map[string]any{"column_id": "todo"})
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "PATCH", path, s.admin.Token, map[string]any{"name": "x"})
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "DELETE", path, s.admin.Token, nil)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "POST", s.columnsPath(), s.admin.Token, map[string]any{"id": "todo", "name": "volta"})
		assert.Equal(t, 409, status, "o id de uma coluna deletada não é reutilizado")
	})
}
