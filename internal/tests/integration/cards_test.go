package integration_test

import (
	"context"
	"strings"
	"testing"

	"kanban-management/internal/database"
	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// cardScenario estende o cenário de boards com um board que tem as colunas "todo" e "doing".
type cardScenario struct {
	boardScenario
	boardID string
}

func newCardScenario(t *testing.T) cardScenario {
	t.Helper()

	s := newBoardScenario(t)

	return cardScenario{
		boardScenario: s,
		boardID:       testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo", "doing"),
	}
}

func (s cardScenario) cardsPath() string { return "/card/board/" + s.boardID }

// cardOrder devolve os títulos dos cards de uma coluna, na ordem da listagem.
func (s cardScenario) cardOrder(t *testing.T, columnID string) []string {
	t.Helper()

	status, env := s.do(t, "GET", s.cardsPath()+"?limit=100"+"&column_id="+columnID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	var titles []string
	for _, card := range env.List() {
		if card["column_id"] == columnID {
			titles = append(titles, card["title"].(string))
		}
	}

	return titles
}

func TestCreateCard_PositionsGrowBy1000AndListIsOrdered(t *testing.T) {
	s := newCardScenario(t)

	for i, title := range []string{"um", "dois", "tres"} {
		status, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": title, "column_id": "todo"})

		require.Equal(t, 201, status)
		assert.Equal(t, float64((i+1)*1000), env.Map()["position"])
		assert.Equal(t, s.projectID, env.Map()["project_id"])
		assert.Equal(t, s.boardID, env.Map()["board_id"])
		assert.Equal(t, s.admin.ID, env.Map()["created_by"])
		assert.NotEmpty(t, env.Map()["id"])
	}

	assert.Equal(t, []string{"um", "dois", "tres"}, s.cardOrder(t, "todo"))

	t.Run("cada coluna tem a sua própria sequência", func(t *testing.T) {
		_, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": "outra", "column_id": "doing"})

		assert.Equal(t, float64(1000), env.Map()["position"])
	})
}

func TestCreateCard_Validation(t *testing.T) {
	s := newCardScenario(t)

	ids := make([]string, 11)
	for i := range ids {
		ids[i] = testutil.CreateUser(t, s.app).ID
	}

	cases := []struct {
		name       string
		body       map[string]any
		wantStatus int
	}{
		{"válido com responsável membro", map[string]any{"title": "ok", "column_id": "todo", "assignee_ids": []string{s.member.ID}}, 201},
		{"título com espaços é aparado", map[string]any{"title": "  ok  ", "column_id": "todo"}, 201},
		{"sem título", map[string]any{"column_id": "todo"}, 400},
		{"título vazio", map[string]any{"title": "", "column_id": "todo"}, 400},
		{"título só com espaços", map[string]any{"title": "   ", "column_id": "todo"}, 400},
		{"título com 201 caracteres", map[string]any{"title": strings.Repeat("a", 201), "column_id": "todo"}, 400},
		{"descrição com 5001 caracteres", map[string]any{"title": "x", "column_id": "todo", "description": strings.Repeat("a", 5001)}, 400},
		{"sem coluna", map[string]any{"title": "x"}, 400},
		{"coluna inexistente", map[string]any{"title": "x", "column_id": "nao-existe"}, 400},
		{"responsável que não é membro", map[string]any{"title": "x", "column_id": "todo", "assignee_ids": []string{s.outsider.ID}}, 400},
		{"responsável duplicado", map[string]any{"title": "x", "column_id": "todo", "assignee_ids": []string{s.member.ID, s.member.ID}}, 400},
		{"responsável com id inválido", map[string]any{"title": "x", "column_id": "todo", "assignee_ids": []string{"abc"}}, 400},
		{"mais de 10 responsáveis", map[string]any{"title": "x", "column_id": "todo", "assignee_ids": ids}, 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", s.cardsPath(), s.admin.Token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}

	t.Run("título é salvo sem espaços nas pontas", func(t *testing.T) {
		_, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": "  aparado  ", "column_id": "todo"})

		assert.Equal(t, "aparado", env.Map()["title"])
	})

	t.Run("corpo inválido", func(t *testing.T) {
		status, _ := s.do(t, "POST", s.cardsPath(), s.admin.Token, "texto")
		assert.Equal(t, 400, status)
	})

	t.Run("ignora position, project_id e created_by enviados pelo cliente", func(t *testing.T) {
		status, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{
			"title": "x", "column_id": "doing", "position": 42, "project_id": "000000000000000000000001", "created_by": s.outsider.ID,
		})

		require.Equal(t, 201, status)
		assert.Equal(t, s.projectID, env.Map()["project_id"])
		assert.Equal(t, s.admin.ID, env.Map()["created_by"])
		assert.NotEqual(t, float64(42), env.Map()["position"])
	})
}

func TestCreateCard_Permissions(t *testing.T) {
	s := newCardScenario(t)
	body := map[string]any{"title": "x", "column_id": "todo"}

	cases := []struct {
		name       string
		path       string
		token      string
		wantStatus int
	}{
		{"member cria", s.cardsPath(), s.member.Token, 201},
		{"manager cria", s.cardsPath(), s.manager.Token, 201},
		{"não-membro recebe 404", s.cardsPath(), s.outsider.Token, 404},
		{"sem autenticação", s.cardsPath(), "", 401},
		{"id de board inválido", "/card/board/invalido", s.admin.Token, 400},
		{"board inexistente", "/card/board/000000000000000000000001", s.admin.Token, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", tc.path, tc.token, body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestGetCards_And_GetCard(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "um")

	cases := []struct {
		name       string
		path       string
		token      string
		wantStatus int
	}{
		{"member lista", s.cardsPath(), s.member.Token, 200},
		{"não-membro não lista", s.cardsPath(), s.outsider.Token, 404},
		{"lista com board inválido", "/card/board/xyz", s.admin.Token, 400},
		{"member vê o card", "/card/" + cardID, s.member.Token, 200},
		{"não-membro não vê o card", "/card/" + cardID, s.outsider.Token, 404},
		{"id de card inválido", "/card/xyz", s.admin.Token, 400},
		{"card inexistente", "/card/000000000000000000000001", s.admin.Token, 404},
		{"sem autenticação", "/card/" + cardID, "", 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "GET", tc.path, tc.token, nil)
			assert.Equal(t, tc.wantStatus, status)
		})
	}

	t.Run("detalhe traz os campos do card", func(t *testing.T) {
		_, env := s.do(t, "GET", "/card/"+cardID, s.member.Token, nil)

		assert.Equal(t, cardID, env.Map()["id"])
		assert.Equal(t, "um", env.Map()["title"])
		assert.Equal(t, "todo", env.Map()["column_id"])
	})
}

func TestPatchCard(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "original")
	path := "/card/" + cardID

	t.Run("member edita título, descrição e responsáveis", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.member.Token, map[string]any{
			"title": "novo", "description": "desc", "assignee_ids": []string{s.manager.ID},
		})
		require.Equal(t, 200, status)

		_, env := s.do(t, "GET", path, s.admin.Token, nil)
		assert.Equal(t, "novo", env.Map()["title"])
		assert.Equal(t, "desc", env.Map()["description"])
		assert.Equal(t, []any{s.manager.ID}, env.Map()["assignee_ids"])
	})

	t.Run("campos fora da allowlist são ignorados", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{
			"title": "so-o-titulo", "column_id": "doing", "position": 9, "project_id": "000000000000000000000001",
			"created_by": s.outsider.ID, "deleted_at": "2020-01-01T00:00:00Z",
		})
		require.Equal(t, 200, status)

		status, env := s.do(t, "GET", path, s.admin.Token, nil)
		require.Equal(t, 200, status, "o card não pode ter sido deletado")
		assert.Equal(t, "so-o-titulo", env.Map()["title"])
		assert.Equal(t, "todo", env.Map()["column_id"])
		assert.Equal(t, float64(1000), env.Map()["position"])
		assert.Equal(t, s.projectID, env.Map()["project_id"])
		assert.Equal(t, s.admin.ID, env.Map()["created_by"])
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"só campos fora da allowlist", path, s.admin.Token, map[string]any{"column_id": "doing"}, 400},
		{"corpo vazio", path, s.admin.Token, map[string]any{}, 400},
		{"JSON inválido", path, s.admin.Token, "texto", 400},
		{"título vazio", path, s.admin.Token, map[string]any{"title": " "}, 400},
		{"responsável que não é membro", path, s.admin.Token, map[string]any{"assignee_ids": []string{s.outsider.ID}}, 400},
		{"não-membro recebe 404", path, s.outsider.Token, map[string]any{"title": "x"}, 404},
		{"id inválido", "/card/xyz", s.admin.Token, map[string]any{"title": "x"}, 400},
		{"card inexistente", "/card/000000000000000000000001", s.admin.Token, map[string]any{"title": "x"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "PATCH", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}

	t.Run("limpar responsáveis com lista vazia", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"assignee_ids": []string{}})
		require.Equal(t, 200, status)

		_, env := s.do(t, "GET", path, s.admin.Token, nil)
		assert.Equal(t, []any{}, env.Map()["assignee_ids"])
	})
}

func TestDeleteCard(t *testing.T) {
	s := newCardScenario(t)

	t.Run("member não deleta card de outro", func(t *testing.T) {
		cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "do admin")

		status, _ := s.do(t, "DELETE", "/card/"+cardID, s.member.Token, nil)

		assert.Equal(t, 403, status)
	})

	t.Run("member deleta o próprio card", func(t *testing.T) {
		cardID := testutil.CreateCard(t, s.app, s.member.Token, s.boardID, "todo", "do member")

		status, _ := s.do(t, "DELETE", "/card/"+cardID, s.member.Token, nil)

		assert.Equal(t, 200, status)
	})

	t.Run("manager deleta card de qualquer um", func(t *testing.T) {
		cardID := testutil.CreateCard(t, s.app, s.member.Token, s.boardID, "todo", "do member 2")

		status, _ := s.do(t, "DELETE", "/card/"+cardID, s.manager.Token, nil)

		assert.Equal(t, 200, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "x")

		status, _ := s.do(t, "DELETE", "/card/"+cardID, s.outsider.Token, nil)

		assert.Equal(t, 404, status)
	})

	t.Run("id inválido", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/card/xyz", s.admin.Token, nil)

		assert.Equal(t, 400, status)
	})

	t.Run("soft delete: some da listagem e do detalhe, mas continua no banco", func(t *testing.T) {
		cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "vai-sumir")

		status, _ := s.do(t, "DELETE", "/card/"+cardID, s.admin.Token, nil)
		require.Equal(t, 200, status)

		status, _ = s.do(t, "GET", "/card/"+cardID, s.admin.Token, nil)
		assert.Equal(t, 404, status)

		assert.NotContains(t, s.cardOrder(t, "todo"), "vai-sumir")

		status, _ = s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"title": "x"})
		assert.Equal(t, 404, status, "card deletado não pode ser editado")

		status, _ = s.do(t, "DELETE", "/card/"+cardID, s.admin.Token, nil)
		assert.Equal(t, 404, status, "deletar duas vezes devolve 404")

		oid, _ := bson.ObjectIDFromHex(cardID)
		var doc bson.M
		err := database.GetCollection("cards").FindOne(context.Background(), bson.M{"_id": oid}).Decode(&doc)
		require.NoError(t, err)
		assert.NotNil(t, doc["deleted_at"], "o documento deve ter deleted_at")
	})
}

func TestDeleteBoard_SoftDeletesItsCards(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "um")
	testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "dois")

	status, _ := s.do(t, "DELETE", "/board/"+s.boardID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	status, _ = s.do(t, "GET", s.cardsPath(), s.admin.Token, nil)
	assert.Equal(t, 404, status, "o board deletado não lista cards")

	status, _ = s.do(t, "GET", "/card/"+cardID, s.admin.Token, nil)
	assert.Equal(t, 404, status)

	boardOID, _ := bson.ObjectIDFromHex(s.boardID)
	total, err := database.GetCollection("cards").CountDocuments(context.Background(), bson.M{"board_id": boardOID})
	require.NoError(t, err)
	deleted, err := database.GetCollection("cards").CountDocuments(context.Background(), bson.M{
		"board_id": boardOID, "deleted_at": bson.M{"$exists": true, "$ne": nil},
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), total)
	assert.Equal(t, total, deleted, "todos os cards do board devem ter deleted_at")
}
