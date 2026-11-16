package integration_test

import (
	"context"
	"testing"

	"kanban-management/internal/database"
	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// threeCards cria "um", "dois" e "tres" na coluna todo (positions 1000, 2000, 3000).
func (s cardScenario) threeCards(t *testing.T) (one, two, three string) {
	t.Helper()

	one = testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "um")
	two = testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "dois")
	three = testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "tres")

	return one, two, three
}

func (s cardScenario) move(t *testing.T, token, cardID string, body any) (int, testutil.Envelope) {
	t.Helper()
	return s.do(t, "PATCH", "/card/"+cardID+"/move", token, body)
}

func TestMoveCard_BetweenTwoCards(t *testing.T) {
	s := newCardScenario(t)
	one, two, three := s.threeCards(t)

	status, env := s.move(t, s.admin.Token, three, map[string]any{
		"column_id": "todo", "before_card_id": one, "after_card_id": two,
	})

	require.Equal(t, 200, status)
	assert.Equal(t, float64(1500), env.Map()["position"])
	assert.Equal(t, []string{"um", "tres", "dois"}, s.cardOrder(t, "todo"))
}

func TestMoveCard_OneNeighbor(t *testing.T) {
	t.Run("só before_card_id: entra logo abaixo dele", func(t *testing.T) {
		s := newCardScenario(t)
		one, _, three := s.threeCards(t)

		status, env := s.move(t, s.admin.Token, three, map[string]any{"column_id": "todo", "before_card_id": one})

		require.Equal(t, 200, status)
		assert.Equal(t, float64(1500), env.Map()["position"], "fica entre o 1º e o 2º")
		assert.Equal(t, []string{"um", "tres", "dois"}, s.cardOrder(t, "todo"))
	})

	t.Run("só before_card_id no último: vai para o fim", func(t *testing.T) {
		s := newCardScenario(t)
		one, _, three := s.threeCards(t)

		status, env := s.move(t, s.admin.Token, one, map[string]any{"column_id": "todo", "before_card_id": three})

		require.Equal(t, 200, status)
		assert.Equal(t, float64(4000), env.Map()["position"])
		assert.Equal(t, []string{"dois", "tres", "um"}, s.cardOrder(t, "todo"))
	})

	t.Run("só after_card_id no primeiro: vai para o início", func(t *testing.T) {
		s := newCardScenario(t)
		one, _, three := s.threeCards(t)

		status, env := s.move(t, s.admin.Token, three, map[string]any{"column_id": "todo", "after_card_id": one})

		require.Equal(t, 200, status)
		assert.Equal(t, float64(500), env.Map()["position"])
		assert.Equal(t, []string{"tres", "um", "dois"}, s.cardOrder(t, "todo"))
	})

	t.Run("só after_card_id no meio: entra logo acima dele", func(t *testing.T) {
		s := newCardScenario(t)
		_, _, three := s.threeCards(t)
		two := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "dois-b")

		status, env := s.move(t, s.admin.Token, two, map[string]any{"column_id": "todo", "after_card_id": three})

		require.Equal(t, 200, status)
		assert.Equal(t, float64(2500), env.Map()["position"], "fica entre o 2º e o 3º")
		assert.Equal(t, []string{"um", "dois", "dois-b", "tres"}, s.cardOrder(t, "todo"))
	})
}

func TestMoveCard_NoNeighbors(t *testing.T) {
	t.Run("para coluna vazia", func(t *testing.T) {
		s := newCardScenario(t)
		one, _, _ := s.threeCards(t)

		status, env := s.move(t, s.admin.Token, one, map[string]any{"column_id": "doing"})

		require.Equal(t, 200, status)
		assert.Equal(t, "doing", env.Map()["column_id"])
		assert.Equal(t, float64(1000), env.Map()["position"])
		assert.Equal(t, []string{"dois", "tres"}, s.cardOrder(t, "todo"))
		assert.Equal(t, []string{"um"}, s.cardOrder(t, "doing"))
	})

	t.Run("para o fim de coluna com cards", func(t *testing.T) {
		s := newCardScenario(t)
		one, _, _ := s.threeCards(t)
		testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "a")
		testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "b")

		status, env := s.move(t, s.admin.Token, one, map[string]any{"column_id": "doing"})

		require.Equal(t, 200, status)
		assert.Equal(t, float64(3000), env.Map()["position"])
		assert.Equal(t, []string{"a", "b", "um"}, s.cardOrder(t, "doing"))
	})

	t.Run("dentro da mesma coluna, para o fim (ignora o próprio card)", func(t *testing.T) {
		s := newCardScenario(t)
		one, _, _ := s.threeCards(t)

		status, env := s.move(t, s.admin.Token, one, map[string]any{"column_id": "todo"})

		require.Equal(t, 200, status)
		assert.Equal(t, float64(4000), env.Map()["position"])
		assert.Equal(t, []string{"dois", "tres", "um"}, s.cardOrder(t, "todo"))
	})
}

func TestMoveCard_RenumbersWhenThereIsNoRoomLeft(t *testing.T) {
	s := newCardScenario(t)
	one, two, three := s.threeCards(t)

	// Aperta o espaço entre o 1º e o 2º card direto no banco (gap de 1e-7).
	oneOID, _ := bson.ObjectIDFromHex(one)
	twoOID, _ := bson.ObjectIDFromHex(two)
	cards := database.GetCollection("cards")
	_, err := cards.UpdateByID(context.Background(), oneOID, bson.M{"$set": bson.M{"position": 1000.0}})
	require.NoError(t, err)
	_, err = cards.UpdateByID(context.Background(), twoOID, bson.M{"$set": bson.M{"position": 1000.0000001}})
	require.NoError(t, err)

	status, env := s.move(t, s.admin.Token, three, map[string]any{
		"column_id": "todo", "before_card_id": one, "after_card_id": two,
	})

	require.Equal(t, 200, status)
	assert.Equal(t, []string{"um", "tres", "dois"}, s.cardOrder(t, "todo"), "a ordem tem que ser a pedida")

	_, list := s.do(t, "GET", s.cardsPath()+"?column_id=todo", s.admin.Token, nil)
	positions := map[string]float64{}
	for _, card := range list.List() {
		positions[card["title"].(string)] = card["position"].(float64)
	}

	// Renumerada (um=1000, dois=2000, tres=3000) e o "tres" entra entre o "um" e o "dois".
	assert.Equal(t, float64(1000), positions["um"], "a coluna foi renumerada de 1000 em 1000")
	assert.Equal(t, float64(2000), positions["dois"])
	assert.Equal(t, float64(1500), positions["tres"])
	assert.Equal(t, float64(1500), env.Map()["position"])
}

func TestMoveCard_Validation(t *testing.T) {
	s := newCardScenario(t)
	one, two, three := s.threeCards(t)
	doingCard := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "doing-1")

	otherBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo")
	otherBoardCard := testutil.CreateCard(t, s.app, s.admin.Token, otherBoard, "todo", "outro-board")

	cases := []struct {
		name       string
		body       any
		wantStatus int
	}{
		{"coluna que não existe", map[string]any{"column_id": "nao-existe"}, 400},
		{"sem column_id", map[string]any{"before_card_id": one}, 400},
		{"corpo inválido", "texto", 400},
		{"before_card_id com id inválido", map[string]any{"column_id": "todo", "before_card_id": "abc"}, 400},
		{"before_card_id de outra coluna", map[string]any{"column_id": "todo", "before_card_id": doingCard}, 400},
		{"before_card_id de outro board", map[string]any{"column_id": "todo", "before_card_id": otherBoardCard}, 400},
		{"after_card_id inexistente", map[string]any{"column_id": "todo", "after_card_id": "000000000000000000000001"}, 400},
		{"before_card_id é o próprio card", map[string]any{"column_id": "todo", "before_card_id": three}, 400},
		{"before abaixo do after", map[string]any{"column_id": "todo", "before_card_id": two, "after_card_id": one}, 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.move(t, s.admin.Token, three, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}

	assert.Equal(t, []string{"um", "dois", "tres"}, s.cardOrder(t, "todo"), "pedidos inválidos não mudam nada")
}

func TestMoveCard_Permissions(t *testing.T) {
	s := newCardScenario(t)
	one, _, _ := s.threeCards(t)

	t.Run("member move", func(t *testing.T) {
		status, _ := s.move(t, s.member.Token, one, map[string]any{"column_id": "doing"})
		assert.Equal(t, 200, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.move(t, s.outsider.Token, one, map[string]any{"column_id": "todo"})
		assert.Equal(t, 404, status)
	})

	t.Run("sem autenticação", func(t *testing.T) {
		status, _ := s.move(t, "", one, map[string]any{"column_id": "todo"})
		assert.Equal(t, 401, status)
	})

	t.Run("id de card inválido", func(t *testing.T) {
		status, _ := s.move(t, s.admin.Token, "xyz", map[string]any{"column_id": "todo"})
		assert.Equal(t, 400, status)
	})

	t.Run("card inexistente", func(t *testing.T) {
		status, _ := s.move(t, s.admin.Token, "000000000000000000000001", map[string]any{"column_id": "todo"})
		assert.Equal(t, 404, status)
	})
}
