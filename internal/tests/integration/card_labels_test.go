package integration_test

import (
	"fmt"
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func labelIDs(card map[string]any) []string {
	raw, _ := card["label_ids"].([]any)

	ids := make([]string, 0, len(raw))
	for _, id := range raw {
		ids = append(ids, id.(string))
	}

	return ids
}

func TestApplyAndRemoveLabel(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "x")
	bug := s.newLabel(t, "Bug")
	front := s.newLabel(t, "Frontend")
	path := func(label string) string { return "/card/" + cardID + "/labels/" + label }

	t.Run("card novo não tem labels (lista vazia, não null)", func(t *testing.T) {
		card := s.cardJSON(t, cardID)

		assert.Equal(t, []any{}, card["label_ids"])
	})

	t.Run("member aplica a label", func(t *testing.T) {
		status, env := s.do(t, "POST", path(bug), s.member.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{bug}, labelIDs(env.Map()))
	})

	t.Run("aplicar a mesma label duas vezes continua com uma só", func(t *testing.T) {
		status, env := s.do(t, "POST", path(bug), s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{bug}, labelIDs(env.Map()))
	})

	t.Run("aplica uma segunda label", func(t *testing.T) {
		status, env := s.do(t, "POST", path(front), s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.ElementsMatch(t, []string{bug, front}, labelIDs(env.Map()))
	})

	t.Run("remove uma label; remover de novo é 200 sem alterar", func(t *testing.T) {
		status, env := s.do(t, "DELETE", path(bug), s.member.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{front}, labelIDs(env.Map()))

		status, env = s.do(t, "DELETE", path(bug), s.member.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{front}, labelIDs(env.Map()))
	})

	t.Run("label de outro board devolve 400", func(t *testing.T) {
		otherBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo")
		status, env := s.do(t, "POST", "/label/board/"+otherBoard, s.admin.Token, map[string]any{"name": "Outra"})
		require.Equal(t, 201, status)

		status, _ = s.do(t, "POST", path(env.Map()["id"].(string)), s.admin.Token, nil)

		assert.Equal(t, 400, status)
	})

	t.Run("label inexistente ou com id inválido", func(t *testing.T) {
		status, _ := s.do(t, "POST", path("000000000000000000000001"), s.admin.Token, nil)
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "POST", path("xyz"), s.admin.Token, nil)
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "DELETE", path("xyz"), s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("não-membro recebe 404 e sem autenticação 401", func(t *testing.T) {
		status, _ := s.do(t, "POST", path(bug), s.outsider.Token, nil)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "DELETE", path(bug), s.outsider.Token, nil)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "POST", path(bug), "", nil)
		assert.Equal(t, 401, status)
	})

	t.Run("card inexistente", func(t *testing.T) {
		status, _ := s.do(t, "POST", "/card/000000000000000000000001/labels/"+bug, s.admin.Token, nil)
		assert.Equal(t, 404, status)
	})
}

func TestApplyLabel_LimitOf10PerCard(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "x")

	for i := 0; i < 10; i++ {
		id := s.newLabel(t, fmt.Sprintf("l%d", i))
		status, _ := s.do(t, "POST", "/card/"+cardID+"/labels/"+id, s.admin.Token, nil)
		require.Equal(t, 200, status, "label %d", i)
	}

	extra := s.newLabel(t, "extra")
	status, _ := s.do(t, "POST", "/card/"+cardID+"/labels/"+extra, s.admin.Token, nil)

	assert.Equal(t, 400, status)
	assert.Len(t, labelIDs(s.cardJSON(t, cardID)), 10)
}

func TestCardLabelIDs_OnCreateAndPatch(t *testing.T) {
	s := newCardScenario(t)
	bug := s.newLabel(t, "Bug")
	front := s.newLabel(t, "Frontend")

	otherBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo")
	_, otherLabel := s.do(t, "POST", "/label/board/"+otherBoard, s.admin.Token, map[string]any{"name": "Outra"})
	foreign := otherLabel.Map()["id"].(string)

	eleven := make([]string, 11)
	for i := range eleven {
		eleven[i] = s.newLabel(t, fmt.Sprintf("n%d", i))
	}

	t.Run("cria card com labels", func(t *testing.T) {
		status, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{
			"title": "x", "column_id": "todo", "label_ids": []string{bug, front},
		})

		require.Equal(t, 201, status)
		assert.ElementsMatch(t, []string{bug, front}, labelIDs(env.Map()))
	})

	createCases := []struct {
		name string
		ids  []string
	}{
		{"label de outro board", []string{foreign}},
		{"label inexistente", []string{"000000000000000000000001"}},
		{"labels duplicadas", []string{bug, bug}},
		{"mais de 10 labels", eleven},
	}

	for _, tc := range createCases {
		t.Run("criar: "+tc.name+" devolve 400", func(t *testing.T) {
			status, _ := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{
				"title": "x", "column_id": "todo", "label_ids": tc.ids,
			})

			assert.Equal(t, 400, status)
		})
	}

	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "patch")

	t.Run("PATCH substitui a lista inteira", func(t *testing.T) {
		status, env := s.do(t, "PATCH", "/card/"+cardID, s.member.Token, map[string]any{"label_ids": []string{bug}})
		require.Equal(t, 200, status)
		assert.Equal(t, []string{bug}, labelIDs(env.Map()))

		status, env = s.do(t, "PATCH", "/card/"+cardID, s.member.Token, map[string]any{"label_ids": []string{front}})
		require.Equal(t, 200, status)
		assert.Equal(t, []string{front}, labelIDs(env.Map()), "substituiu, não somou")
	})

	t.Run("PATCH com lista vazia limpa as labels", func(t *testing.T) {
		status, env := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"label_ids": []string{}})

		require.Equal(t, 200, status)
		assert.Equal(t, []any{}, env.Map()["label_ids"])
	})

	for _, tc := range createCases {
		t.Run("PATCH: "+tc.name+" devolve 400", func(t *testing.T) {
			status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"label_ids": tc.ids})

			assert.Equal(t, 400, status)
		})
	}
}
