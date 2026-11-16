package integration_test

import (
	"testing"
	"time"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s cardScenario) createCardWithDue(t *testing.T, title string, due any) (int, testutil.Envelope) {
	t.Helper()

	return s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": title, "column_id": "todo", "due_date": due})
}

func inDays(days int) string {
	return time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339)
}

func TestCreateCard_DueDate(t *testing.T) {
	s := newCardScenario(t)

	cases := []struct {
		name       string
		due        any
		wantStatus int
		wantDue    any
	}{
		{"só a data vira fim do dia em UTC", "2026-12-01", 201, "2026-12-01T23:59:59Z"},
		{"RFC 3339 com fuso vira UTC", "2026-12-01T10:00:00-03:00", 201, "2026-12-01T13:00:00Z"},
		{"RFC 3339 em UTC", "2026-12-01T18:00:00Z", 201, "2026-12-01T18:00:00Z"},
		{"null é o mesmo que sem data", nil, 201, nil},
		{"data no passado é permitida", "2025-01-15", 201, "2025-01-15T23:59:59Z"},
		{"texto inválido", "amanhã", 400, nil},
		{"antes de 2000", "1990-05-01", 400, nil},
		{"mais de 10 anos no futuro", "2099-01-01", 400, nil},
		{"tipo errado", 20261201, 400, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, env := s.createCardWithDue(t, "x", tc.due)

			assert.Equal(t, tc.wantStatus, status)

			if tc.wantStatus == 201 {
				assert.Equal(t, tc.wantDue, env.Map()["due_date"])
			}
		})
	}

	t.Run("sem due_date", func(t *testing.T) {
		status, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{"title": "x", "column_id": "todo"})

		require.Equal(t, 201, status)
		assert.Nil(t, env.Map()["due_date"])
		assert.Equal(t, false, env.Map()["is_overdue"])
	})
}

func TestPatchCard_DueDate(t *testing.T) {
	s := newCardScenario(t)
	cardID := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "x")
	path := "/card/" + cardID

	t.Run("define a data e devolve o card atualizado", func(t *testing.T) {
		status, env := s.do(t, "PATCH", path, s.member.Token, map[string]any{"due_date": "2026-12-01"})

		require.Equal(t, 200, status)
		assert.Equal(t, "2026-12-01T23:59:59Z", env.Map()["due_date"])
		assert.Equal(t, cardID, env.Map()["id"])
		assert.Equal(t, "2026-12-01T23:59:59Z", s.cardJSON(t, cardID)["due_date"])
	})

	t.Run("data inválida devolve 400 e mantém a anterior", func(t *testing.T) {
		for _, bad := range []any{"amanhã", "1990-01-01", 123, true} {
			status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"due_date": bad})
			assert.Equal(t, 400, status, "due_date=%v", bad)
		}

		assert.Equal(t, "2026-12-01T23:59:59Z", s.cardJSON(t, cardID)["due_date"])
	})

	t.Run("troca a data junto com outro campo", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"due_date": "2026-12-05", "title": "novo"})

		require.Equal(t, 200, status)
		card := s.cardJSON(t, cardID)
		assert.Equal(t, "2026-12-05T23:59:59Z", card["due_date"])
		assert.Equal(t, "novo", card["title"])
	})

	t.Run("null remove a data", func(t *testing.T) {
		status, env := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"due_date": nil})

		require.Equal(t, 200, status)
		assert.Nil(t, env.Map()["due_date"])
		assert.Nil(t, s.cardJSON(t, cardID)["due_date"])
	})

	t.Run("null sozinho é um pedido válido mesmo sem data antes", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"due_date": nil})

		assert.Equal(t, 200, status)
	})

	t.Run("completed_at, completed_by e is_overdue enviados pelo cliente são ignorados", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.admin.Token, map[string]any{"completed_at": "2026-01-01T00:00:00Z"})
		assert.Equal(t, 400, status, "só campos fora da allowlist")

		status, _ = s.do(t, "PATCH", path, s.admin.Token, map[string]any{
			"title": "com-extras", "completed_at": "2026-01-01T00:00:00Z", "completed_by": s.outsider.ID, "is_overdue": true,
		})
		require.Equal(t, 200, status)

		card := s.cardJSON(t, cardID)
		assert.Nil(t, card["completed_at"])
		assert.Nil(t, card["completed_by"])
		assert.Equal(t, false, card["is_overdue"])
	})
}

func TestListCards_HasIsOverdue(t *testing.T) {
	s := newCardScenario(t)

	s.createCardWithDue(t, "atrasado", "2025-01-15")
	s.createCardWithDue(t, "no-prazo", inDays(30))
	s.createCardWithDue(t, "sem-data", nil)

	status, env := s.do(t, "GET", s.cardsPath()+"?column_id=todo", s.admin.Token, nil)
	require.Equal(t, 200, status)

	overdue := map[string]any{}
	for _, card := range env.List() {
		overdue[card["title"].(string)] = card["is_overdue"]
	}

	assert.Equal(t, true, overdue["atrasado"])
	assert.Equal(t, false, overdue["no-prazo"])
	assert.Equal(t, false, overdue["sem-data"])
}
