package integration_test

import (
	"fmt"
	"strings"
	"testing"

	"kanban-management/internal/services"
	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s cardScenario) labelsPath() string { return "/label/board/" + s.boardID }

func (s cardScenario) createLabel(t *testing.T, token, name string) (int, testutil.Envelope) {
	t.Helper()

	return s.do(t, "POST", s.labelsPath(), token, map[string]any{"name": name})
}

func (s cardScenario) newLabel(t *testing.T, name string) string {
	t.Helper()

	status, env := s.createLabel(t, s.admin.Token, name)
	require.Equal(t, 201, status, "create label %q: %v", name, env.Error)

	return env.Map()["id"].(string)
}

func TestCreateLabel(t *testing.T) {
	s := newCardScenario(t)

	t.Run("admin cria com cor e o nome é aparado", func(t *testing.T) {
		status, env := s.do(t, "POST", s.labelsPath(), s.admin.Token, map[string]any{"name": "  Bug  ", "color": "#FF0000"})

		require.Equal(t, 201, status)
		label := env.Map()
		assert.Equal(t, "Bug", label["name"])
		assert.Equal(t, "#FF0000", label["color"])
		assert.Equal(t, s.boardID, label["board_id"])
		assert.Equal(t, s.projectID, label["project_id"])
		assert.Equal(t, s.admin.ID, label["created_by"])
		assert.NotContains(t, label, "name_normalized", "campo interno")
		assert.NotContains(t, label, "active", "campo interno")
	})

	t.Run("sem cor recebe uma da paleta", func(t *testing.T) {
		status, env := s.createLabel(t, s.manager.Token, "Frontend")

		require.Equal(t, 201, status)
		assert.Contains(t, services.LabelPalette, env.Map()["color"])
	})

	t.Run("mesmo nome com caixa diferente no mesmo board devolve 409", func(t *testing.T) {
		status, env := s.createLabel(t, s.admin.Token, "bug")
		assert.Equal(t, 409, status)
		assert.Equal(t, "Label name already exists on this board", *env.Error)

		status, _ = s.createLabel(t, s.admin.Token, " BUG ")
		assert.Equal(t, 409, status)
	})

	t.Run("o mesmo nome em outro board é permitido", func(t *testing.T) {
		otherBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo")

		status, _ := s.do(t, "POST", "/label/board/"+otherBoard, s.admin.Token, map[string]any{"name": "Bug"})

		assert.Equal(t, 201, status)
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"cor inválida (nome)", s.labelsPath(), s.admin.Token, map[string]any{"name": "a", "color": "red"}, 400},
		{"cor curta", s.labelsPath(), s.admin.Token, map[string]any{"name": "a", "color": "#fff"}, 400},
		{"sem nome", s.labelsPath(), s.admin.Token, map[string]any{"color": "#ffffff"}, 400},
		{"nome em branco", s.labelsPath(), s.admin.Token, map[string]any{"name": "   "}, 400},
		{"nome com 31 caracteres", s.labelsPath(), s.admin.Token, map[string]any{"name": strings.Repeat("a", 31)}, 400},
		{"corpo inválido", s.labelsPath(), s.admin.Token, "texto", 400},
		{"member não cria", s.labelsPath(), s.member.Token, map[string]any{"name": "x"}, 403},
		{"não-membro recebe 404", s.labelsPath(), s.outsider.Token, map[string]any{"name": "x"}, 404},
		{"sem autenticação", s.labelsPath(), "", map[string]any{"name": "x"}, 401},
		{"id de board inválido", "/label/board/xyz", s.admin.Token, map[string]any{"name": "x"}, 400},
		{"board inexistente", "/label/board/000000000000000000000001", s.admin.Token, map[string]any{"name": "x"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestCreateLabel_LimitOf50PerBoard(t *testing.T) {
	s := newCardScenario(t)

	for i := 0; i < services.MaxLabelsPerBoard; i++ {
		status, env := s.createLabel(t, s.admin.Token, fmt.Sprintf("label-%02d", i))
		require.Equal(t, 201, status, "label %d: %v", i, env.Error)
	}

	status, _ := s.createLabel(t, s.admin.Token, "label-51")
	assert.Equal(t, 400, status)

	// Deletar uma abre vaga.
	_, env := s.do(t, "GET", s.labelsPath(), s.admin.Token, nil)
	status, _ = s.do(t, "DELETE", "/label/"+env.List()[0]["id"].(string), s.admin.Token, nil)
	require.Equal(t, 200, status)

	status, _ = s.createLabel(t, s.admin.Token, "label-51")
	assert.Equal(t, 201, status)
}

func TestGetLabels(t *testing.T) {
	s := newCardScenario(t)
	s.newLabel(t, "zebra")
	s.newLabel(t, "Alfa")
	deletedID := s.newLabel(t, "apagada")

	status, _ := s.do(t, "DELETE", "/label/"+deletedID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	t.Run("member lista ordenado por nome, sem as deletadas", func(t *testing.T) {
		status, env := s.do(t, "GET", s.labelsPath(), s.member.Token, nil)

		require.Equal(t, 200, status)

		var names []string
		for _, label := range env.List() {
			names = append(names, label["name"].(string))
		}

		assert.Equal(t, []string{"Alfa", "zebra"}, names)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "GET", s.labelsPath(), s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("id de board inválido", func(t *testing.T) {
		status, _ := s.do(t, "GET", "/label/board/xyz", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("board sem labels devolve lista vazia", func(t *testing.T) {
		emptyBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo")

		status, env := s.do(t, "GET", "/label/board/"+emptyBoard, s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Empty(t, env.List())
		assert.JSONEq(t, "[]", string(env.Data))
	})
}

func TestPatchLabel(t *testing.T) {
	s := newCardScenario(t)
	bugID := s.newLabel(t, "Bug")
	s.newLabel(t, "Feature")
	path := "/label/" + bugID

	t.Run("edita nome e cor", func(t *testing.T) {
		status, env := s.do(t, "PATCH", path, s.manager.Token, map[string]any{"name": "Defeito", "color": "#00ff00"})

		require.Equal(t, 200, status)
		assert.Equal(t, "Defeito", env.Map()["name"])
		assert.Equal(t, "#00ff00", env.Map()["color"])
	})

	t.Run("mudar só a caixa do próprio nome é permitido", func(t *testing.T) {
		status, env := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"name": "DEFEITO"})

		require.Equal(t, 200, status)
		assert.Equal(t, "DEFEITO", env.Map()["name"])
	})

	t.Run("renomear para o nome de outra label devolve 409", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"name": "feature"})

		assert.Equal(t, 409, status)
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"cor inválida", path, s.admin.Token, map[string]any{"color": "verde"}, 400},
		{"nome em branco", path, s.admin.Token, map[string]any{"name": " "}, 400},
		{"só campos fora da allowlist", path, s.admin.Token, map[string]any{"board_id": s.boardID, "active": false}, 400},
		{"JSON inválido", path, s.admin.Token, "texto", 400},
		{"member não edita", path, s.member.Token, map[string]any{"name": "x"}, 403},
		{"não-membro recebe 404", path, s.outsider.Token, map[string]any{"name": "x"}, 404},
		{"id inválido", "/label/xyz", s.admin.Token, map[string]any{"name": "x"}, 400},
		{"label inexistente", "/label/000000000000000000000001", s.admin.Token, map[string]any{"name": "x"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "PATCH", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestDeleteLabel(t *testing.T) {
	s := newCardScenario(t)
	labelID := s.newLabel(t, "Bug")

	t.Run("member não deleta", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/label/"+labelID, s.member.Token, nil)
		assert.Equal(t, 403, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/label/"+labelID, s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("id inválido", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/label/xyz", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("admin deleta; o nome fica livre de novo", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/label/"+labelID, s.admin.Token, nil)
		require.Equal(t, 200, status)

		status, _ = s.do(t, "DELETE", "/label/"+labelID, s.admin.Token, nil)
		assert.Equal(t, 404, status, "deletar duas vezes devolve 404")

		status, _ = s.do(t, "PATCH", "/label/"+labelID, s.admin.Token, map[string]any{"name": "x"})
		assert.Equal(t, 404, status, "label deletada não pode ser editada")

		status, env := s.createLabel(t, s.admin.Token, "bug")
		assert.Equal(t, 201, status, "recriar o nome depois de deletar")
		assert.NotEqual(t, labelID, env.Map()["id"])
	})
}
