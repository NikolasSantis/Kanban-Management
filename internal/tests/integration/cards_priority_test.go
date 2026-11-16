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

func (s cardScenario) cardJSON(t *testing.T, cardID string) map[string]any {
	t.Helper()

	status, env := s.do(t, "GET", "/card/"+cardID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	return env.Map()
}

func TestCreateCard_Priority(t *testing.T) {
	s := newCardScenario(t)

	cases := []struct {
		name         string
		priority     any
		wantStatus   int
		wantPriority string
	}{
		{"sem prioridade vira medium", nil, 201, "medium"},
		{"urgent", "urgent", 201, "urgent"},
		{"maiúsculas são normalizadas", "HIGH", 201, "high"},
		{"low", "low", 201, "low"},
		{"valor inválido", "critical", 400, ""},
		{"string vazia", "", 400, ""},
		{"tipo errado", 3, 400, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := map[string]any{"title": "x", "column_id": "todo"}
			if tc.priority != nil {
				body["priority"] = tc.priority
			}

			status, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, body)

			assert.Equal(t, tc.wantStatus, status)

			if tc.wantStatus == 201 {
				assert.Equal(t, tc.wantPriority, env.Map()["priority"])
				assert.NotContains(t, env.Map(), "priority_level", "o nível é interno")
			} else if _, isString := tc.priority.(string); isString {
				assert.Equal(t, "Invalid priority. Allowed: low, medium, high, urgent", *env.Error)
			}
		})
	}
}

func TestPatchCard_Priority(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "x")
	oid, _ := bson.ObjectIDFromHex(cardID)

	level := func() int32 {
		var doc bson.M
		require.NoError(t, database.GetCollection("cards").FindOne(context.Background(), bson.M{"_id": oid}).Decode(&doc))
		return doc["priority_level"].(int32)
	}

	require.Equal(t, int32(2), level(), "padrão medium = nível 2")

	t.Run("muda para low e atualiza o nível junto", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.member.Token, map[string]any{"priority": "low"})

		require.Equal(t, 200, status)
		assert.Equal(t, "low", s.cardJSON(t, cardID)["priority"])
		assert.Equal(t, int32(1), level())
	})

	t.Run("urgent em maiúsculas", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"priority": "URGENT"})

		require.Equal(t, 200, status)
		assert.Equal(t, "urgent", s.cardJSON(t, cardID)["priority"])
		assert.Equal(t, int32(4), level())
	})

	t.Run("valor inválido devolve 400 e não muda nada", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"priority": "super"})

		assert.Equal(t, 400, status)
		assert.Equal(t, "urgent", s.cardJSON(t, cardID)["priority"])
	})

	t.Run("priority_level enviado pelo cliente é ignorado", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"priority_level": 1, "title": "novo"})

		require.Equal(t, 200, status)
		assert.Equal(t, int32(4), level(), "o nível só muda junto com a prioridade")
	})

	t.Run("só priority_level devolve 400", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"priority_level": 1})

		assert.Equal(t, 400, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.outsider.Token, map[string]any{"priority": "low"})

		assert.Equal(t, 404, status)
	})
}
