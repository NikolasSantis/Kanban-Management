package integration_test

import (
	"encoding/json"
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagination_EveryListRouteUsesItemsAndPagination(t *testing.T) {
	s := newAttachmentScenario(t)
	s.comment(t, s.admin.Token, "oi")
	s.newLabel(t, "Bug")
	s.uploadOK(t, s.admin.Token, "a.png", pngBytes)

	routes := []struct {
		name string
		path string
	}{
		{"meus workspaces (dono)", "/workspace/"},
		{"workspaces em que sou membro", "/workspace/member"},
		{"membros do workspace", "/workspace/member/" + s.workspaceID},
		{"meus projetos", "/project/"},
		{"membros do projeto", "/project/member/" + s.projectID},
		{"boards do projeto", "/board/project/" + s.projectID},
		{"cards de uma coluna", "/card/board/" + s.boardID + "?column_id=todo"},
		{"labels do board", "/label/board/" + s.boardID},
		{"comentários do card", "/comment/card/" + s.cardID},
		{"anexos do card", "/attachment/card/" + s.cardID},
	}

	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			status, env := s.do(t, "GET", route.path, s.admin.Token, nil)

			require.Equal(t, 200, status)
			require.NotNil(t, env.Pagination(), "data precisa ter {items, pagination}")
			assert.NotNil(t, env.Pagination()["total"])
			assert.NotNil(t, env.Pagination()["total_pages"])
			assert.NotNil(t, env.Pagination()["has_next"])
			assert.True(t, len(env.ItemsRaw()) > 0 && env.ItemsRaw()[0] == '[', "items sempre é um array")
		})
	}
}

func TestPagination_CardsPerColumn(t *testing.T) {
	s := newCardScenario(t)

	for _, title := range []string{"t1", "t2", "t3"} {
		testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", title)
	}
	testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "d1")

	t.Run("sem column_id: a primeira página de cada coluna", func(t *testing.T) {
		status, env := s.do(t, "GET", s.cardsPath()+"?limit=2", s.admin.Token, nil)

		require.Equal(t, 200, status)

		var data struct {
			Columns map[string]struct {
				Items      []map[string]any `json:"items"`
				Pagination map[string]any   `json:"pagination"`
			} `json:"columns"`
		}
		require.NoError(t, jsonUnmarshal(env.Data, &data))

		require.Len(t, data.Columns, 2, "uma entrada por coluna ativa")
		assert.Len(t, data.Columns["todo"].Items, 2)
		assert.Equal(t, float64(3), data.Columns["todo"].Pagination["total"])
		assert.Equal(t, true, data.Columns["todo"].Pagination["has_next"])
		assert.Len(t, data.Columns["doing"].Items, 1)
		assert.Equal(t, false, data.Columns["doing"].Pagination["has_next"])
		assert.Equal(t, "t1", data.Columns["todo"].Items[0]["title"], "ordenado por position")
	})

	t.Run("com column_id: pagina só aquela coluna", func(t *testing.T) {
		_, first := s.do(t, "GET", s.cardsPath()+"?column_id=todo&limit=2&page=1", s.admin.Token, nil)
		_, second := s.do(t, "GET", s.cardsPath()+"?column_id=todo&limit=2&page=2", s.admin.Token, nil)

		require.Len(t, first.List(), 2)
		require.Len(t, second.List(), 1)
		assert.Equal(t, "t1", first.List()[0]["title"])
		assert.Equal(t, "t2", first.List()[1]["title"])
		assert.Equal(t, "t3", second.List()[0]["title"])
		assert.Equal(t, float64(2), second.Pagination()["total_pages"])
	})

	t.Run("coluna deletada não aparece; coluna inexistente devolve 400", func(t *testing.T) {
		status, _ := s.do(t, "GET", s.cardsPath()+"?column_id=nao-existe", s.admin.Token, nil)
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "GET", s.cardsPath()+"?page=abc", s.admin.Token, nil)
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "GET", s.cardsPath(), s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})
}

func jsonUnmarshal(raw []byte, v any) error {
	return json.Unmarshal(raw, v)
}
