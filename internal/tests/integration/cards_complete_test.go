package integration_test

import (
	"testing"
	"time"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteAndReopenCard(t *testing.T) {
	s := newCardScenario(t)

	status, env := s.createCardWithDue(t, "atrasado", "2025-01-15")
	require.Equal(t, 201, status)
	cardID := env.Map()["id"].(string)

	assert.Equal(t, true, env.Map()["is_overdue"], "data no passado e não concluído")
	assert.Equal(t, true, s.cardJSON(t, cardID)["is_overdue"])

	var completedAt any

	t.Run("complete marca concluído e tira o atraso", func(t *testing.T) {
		status, env := s.do(t, "POST", "/card/"+cardID+"/complete", s.member.Token, nil)

		require.Equal(t, 200, status)
		assert.NotNil(t, env.Map()["completed_at"])
		assert.Equal(t, s.member.ID, env.Map()["completed_by"])
		assert.Equal(t, false, env.Map()["is_overdue"])

		completedAt = env.Map()["completed_at"]
	})

	t.Run("complete de novo é idempotente (não altera nada)", func(t *testing.T) {
		status, env := s.do(t, "POST", "/card/"+cardID+"/complete", s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, completedAt, env.Map()["completed_at"])
		assert.Equal(t, s.member.ID, env.Map()["completed_by"], "continua sendo quem concluiu primeiro")
	})

	t.Run("reopen remove a conclusão e o atraso volta", func(t *testing.T) {
		status, env := s.do(t, "POST", "/card/"+cardID+"/reopen", s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Nil(t, env.Map()["completed_at"])
		assert.Nil(t, env.Map()["completed_by"])
		assert.Equal(t, true, env.Map()["is_overdue"])
	})

	t.Run("reopen de card não concluído é 200 sem alterar", func(t *testing.T) {
		status, _ := s.do(t, "POST", "/card/"+cardID+"/reopen", s.admin.Token, nil)

		assert.Equal(t, 200, status)
	})

	t.Run("permissões e ids", func(t *testing.T) {
		status, _ := s.do(t, "POST", "/card/"+cardID+"/complete", s.outsider.Token, nil)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "POST", "/card/"+cardID+"/complete", "", nil)
		assert.Equal(t, 401, status)

		status, _ = s.do(t, "POST", "/card/xyz/complete", s.admin.Token, nil)
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "POST", "/card/000000000000000000000001/reopen", s.admin.Token, nil)
		assert.Equal(t, 404, status)
	})
}

func TestGetCardsDue_Calendar(t *testing.T) {
	s := newCardScenario(t)
	path := "/card/project/" + s.projectID + "/due"

	s.createCardWithDue(t, "em-3-dias", inDays(3))
	s.createCardWithDue(t, "em-20-dias", inDays(20))
	s.createCardWithDue(t, "sem-data", nil)
	_, env := s.createCardWithDue(t, "apagado", inDays(2))
	status, _ := s.do(t, "DELETE", "/card/"+env.Map()["id"].(string), s.admin.Token, nil)
	require.Equal(t, 200, status)

	titles := func(env testutil.Envelope) []string {
		var out []string
		for _, card := range env.List() {
			out = append(out, card["title"].(string))
		}
		return out
	}

	t.Run("padrão: próximos 7 dias", func(t *testing.T) {
		status, env := s.do(t, "GET", path, s.member.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{"em-3-dias"}, titles(env), "sem data, apagado e fora do intervalo não entram")
	})

	t.Run("intervalo explícito, ordenado por data", func(t *testing.T) {
		status, env := s.do(t, "GET", path+"?from="+inDays(0)+"&to="+inDays(25), s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{"em-3-dias", "em-20-dias"}, titles(env))
	})

	t.Run("só from usa 7 dias a partir dele", func(t *testing.T) {
		from := time.Now().UTC().AddDate(0, 0, 15).Format("2006-01-02")

		status, env := s.do(t, "GET", path+"?from="+from, s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{"em-20-dias"}, titles(env))
	})

	cases := []struct {
		name       string
		query      string
		token      string
		wantStatus int
	}{
		{"mais de 90 dias", "?from=2026-01-01&to=2026-06-01", s.admin.Token, 400},
		{"to antes de from", "?from=2026-06-01&to=2026-01-01", s.admin.Token, 400},
		{"from inválido", "?from=ontem", s.admin.Token, 400},
		{"to inválido", "?to=amanhã", s.admin.Token, 400},
		{"não-membro recebe 404", "", s.outsider.Token, 404},
		{"sem autenticação", "", "", 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "GET", path+tc.query, tc.token, nil)
			assert.Equal(t, tc.wantStatus, status)
		})
	}

	t.Run("id de projeto inválido", func(t *testing.T) {
		status, _ := s.do(t, "GET", "/card/project/xyz/due", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})
}
