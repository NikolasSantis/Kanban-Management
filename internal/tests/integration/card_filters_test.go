package integration_test

import (
	"net/url"
	"sort"
	"testing"
	"time"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// filterFixture é um board com 12 cards variados.
//
//	título  col    prioridade assignee  due      estado     labels  criador
//	c01     todo   urgent     member    +2d      aberto     -       admin
//	c02     todo   urgent     member    +10d     concluído  -       admin
//	c03     todo   high       admin     -3d      aberto     -       admin
//	c04     doing  high       -         -        aberto     A       admin
//	c05     doing  medium     -         +5d      aberto     A,B     admin
//	c06     doing  low        -         -        aberto     -       member
//	c07     todo   urgent     -         -        aberto     -       admin
//	c08     todo   medium     member    -10d     concluído  -       admin
//	c09     doing  high       -         +30d     aberto     B       admin
//	c10     todo   low        -         +1d      aberto     -       admin
//	c11     todo   medium     -         -        aberto     -       manager
//	c12     doing  urgent     member    +3d      aberto     A       admin
//
// Os títulos de c01 a c06 também têm a palavra "login".
type filterFixture struct {
	cardScenario
	labelA, labelB string
}

func newFilterFixture(t *testing.T) filterFixture {
	t.Helper()

	s := newCardScenario(t)
	f := filterFixture{cardScenario: s, labelA: s.newLabel(t, "A"), labelB: s.newLabel(t, "B")}

	day := func(days int) string { return time.Now().UTC().AddDate(0, 0, days).Format(time.RFC3339) }

	cards := []struct {
		token     string
		title     string
		column    string
		priority  string
		assignee  string
		due       string
		labels    []string
		completed bool
	}{
		{s.admin.Token, "c01 login", "todo", "urgent", s.member.ID, day(2), nil, false},
		{s.admin.Token, "c02 login", "todo", "urgent", s.member.ID, day(10), nil, true},
		{s.admin.Token, "c03 login", "todo", "high", s.admin.ID, day(-3), nil, false},
		{s.admin.Token, "c04 login", "doing", "high", "", "", []string{f.labelA}, false},
		{s.admin.Token, "c05 login", "doing", "medium", "", day(5), []string{f.labelA, f.labelB}, false},
		{s.member.Token, "c06 login", "doing", "low", "", "", nil, false},
		{s.admin.Token, "c07", "todo", "urgent", "", "", nil, false},
		{s.admin.Token, "c08", "todo", "medium", s.member.ID, day(-10), nil, true},
		{s.admin.Token, "c09", "doing", "high", "", day(30), []string{f.labelB}, false},
		{s.admin.Token, "c10", "todo", "low", "", day(1), nil, false},
		{s.manager.Token, "c11", "todo", "medium", "", "", nil, false},
		{s.admin.Token, "c12", "doing", "urgent", s.member.ID, day(3), []string{f.labelA}, false},
	}

	for _, card := range cards {
		body := map[string]any{"title": card.title, "column_id": card.column, "priority": card.priority}

		if card.assignee != "" {
			body["assignee_ids"] = []string{card.assignee}
		}

		if card.due != "" {
			body["due_date"] = card.due
		}

		if card.labels != nil {
			body["label_ids"] = card.labels
		}

		status, env := s.do(t, "POST", s.cardsPath(), card.token, body)
		require.Equal(t, 201, status, "%s: %v", card.title, env.Error)

		if card.completed {
			status, _ = s.do(t, "POST", "/card/"+env.Map()["id"].(string)+"/complete", s.admin.Token, nil)
			require.Equal(t, 200, status)
		}
	}

	return f
}

func (f filterFixture) query(t *testing.T, token, rawQuery string) (int, testutil.Envelope) {
	t.Helper()

	return f.do(t, "GET", f.cardsPath()+"?"+rawQuery, token, nil)
}

// titlesFor devolve os 3 primeiros caracteres do título (c01, c02...) na ordem recebida.
func shortTitles(env testutil.Envelope) []string {
	out := []string{}

	for _, item := range env.List() {
		out = append(out, item["title"].(string)[:3])
	}

	return out
}

func sorted(values []string) []string {
	cp := append([]string{}, values...)
	sort.Strings(cp)

	return cp
}

func TestCardFilters(t *testing.T) {
	f := newFilterFixture(t)

	day := func(days int) string { return time.Now().UTC().AddDate(0, 0, days).Format("2006-01-02") }

	cases := []struct {
		name  string
		token string
		query string
		want  []string
	}{
		{"column_id (vários)", f.admin.Token, "column_id=todo,doing&limit=100", []string{"c01", "c02", "c03", "c04", "c05", "c06", "c07", "c08", "c09", "c10", "c11", "c12"}},
		{"column_id (um)", f.admin.Token, "column_id=doing", []string{"c04", "c05", "c06", "c09", "c12"}},
		{"priority=high,urgent", f.admin.Token, "priority=high,urgent&limit=100", []string{"c01", "c02", "c03", "c04", "c07", "c09", "c12"}},
		{"priority em maiúsculas", f.admin.Token, "priority=URGENT", []string{"c01", "c02", "c07", "c12"}},
		{"label_ids exige TODAS (padrão)", f.admin.Token, "label_ids=" + f.labelA + "," + f.labelB, []string{"c05"}},
		{"label_mode=any aceita qualquer uma", f.admin.Token, "label_ids=" + f.labelA + "," + f.labelB + "&label_mode=any", []string{"c04", "c05", "c09", "c12"}},
		{"label_ids de uma só", f.admin.Token, "label_ids=" + f.labelB, []string{"c05", "c09"}},
		{"assignee_id=me (como member)", f.member.Token, "assignee_id=me", []string{"c01", "c02", "c08", "c12"}},
		{"assignee_id=me (como admin)", f.admin.Token, "assignee_id=me", []string{"c03"}},
		{"assignee_id por id", f.admin.Token, "assignee_id=" + f.member.ID, []string{"c01", "c02", "c08", "c12"}},
		{"assignee_id=none", f.admin.Token, "assignee_id=none&limit=100", []string{"c04", "c05", "c06", "c07", "c09", "c10", "c11"}},
		{"created_by=me (como member)", f.member.Token, "created_by=me", []string{"c06"}},
		{"created_by por id", f.admin.Token, "created_by=" + f.manager.ID, []string{"c11"}},
		{"status=completed", f.admin.Token, "status=completed", []string{"c02", "c08"}},
		{"status=open", f.admin.Token, "status=open&limit=100", []string{"c01", "c03", "c04", "c05", "c06", "c07", "c09", "c10", "c11", "c12"}},
		{"status=all não restringe", f.admin.Token, "status=all&priority=low,medium,high,urgent&limit=100", []string{"c01", "c02", "c03", "c04", "c05", "c06", "c07", "c08", "c09", "c10", "c11", "c12"}},
		{"overdue=true: atrasado e não concluído", f.admin.Token, "overdue=true", []string{"c03"}},
		{"overdue=false: o resto", f.admin.Token, "overdue=false&limit=100", []string{"c01", "c02", "c04", "c05", "c06", "c07", "c08", "c09", "c10", "c11", "c12"}},
		{"due_after + due_before (início e fim do dia)", f.admin.Token, "due_after=" + day(0) + "&due_before=" + day(6), []string{"c01", "c05", "c10", "c12"}},
		{"só due_before", f.admin.Token, "due_before=" + day(-1), []string{"c03", "c08"}},
		{"só due_after", f.admin.Token, "due_after=" + day(9), []string{"c02", "c09"}},
		{"has_due_date=true", f.admin.Token, "has_due_date=true&limit=100", []string{"c01", "c02", "c03", "c05", "c08", "c09", "c10", "c12"}},
		{"has_due_date=false", f.admin.Token, "has_due_date=false", []string{"c04", "c06", "c07", "c11"}},
		{"combinação: urgentes abertos que são meus", f.member.Token, "priority=urgent&assignee_id=me&status=open", []string{"c01", "c12"}},
		{"combinação: prioridade + coluna + label", f.admin.Token, "priority=high&column_id=doing&label_ids=" + f.labelA, []string{"c04"}},
		{"nenhum resultado", f.admin.Token, "priority=low&status=completed", []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, env := f.query(t, tc.token, tc.query)

			require.Equal(t, 200, status, "%v", env.Error)
			assert.Equal(t, sorted(tc.want), sorted(shortTitles(env)))
			assert.Equal(t, float64(len(tc.want)), env.Pagination()["total"], "total reflete o filtro")
		})
	}
}

func TestCardFilters_SortByPriority(t *testing.T) {
	f := newFilterFixture(t)

	_, env := f.query(t, f.admin.Token, "sort=priority&order=desc&limit=100")

	var levels []string
	for _, item := range env.List() {
		levels = append(levels, item["priority"].(string))
	}

	// urgent (4), high (3), medium (2), low (1): a ordem NÃO é a alfabética.
	assert.Equal(t, []string{
		"urgent", "urgent", "urgent", "urgent",
		"high", "high", "high",
		"medium", "medium", "medium",
		"low", "low",
	}, levels)

	_, env = f.query(t, f.admin.Token, "sort=priority&order=asc&limit=100")
	assert.Equal(t, "low", env.List()[0]["priority"])
	assert.Equal(t, "urgent", env.List()[11]["priority"])
}

func TestCardFilters_SortByDueDate_CardsWithoutDateGoLast(t *testing.T) {
	f := newFilterFixture(t)
	withDates := 8

	asc := func(env testutil.Envelope) []string { return shortTitles(env) }

	t.Run("asc: do mais próximo ao mais distante, sem data por último", func(t *testing.T) {
		status, env := f.query(t, f.admin.Token, "sort=due_date&limit=100")

		require.Equal(t, 200, status)
		titles := asc(env)

		assert.Equal(t, []string{"c08", "c03", "c10", "c01", "c12", "c05", "c02", "c09"}, titles[:withDates])
		assert.ElementsMatch(t, []string{"c04", "c06", "c07", "c11"}, titles[withDates:])
	})

	t.Run("desc: os sem data continuam por último", func(t *testing.T) {
		status, env := f.query(t, f.admin.Token, "sort=due_date&order=desc&limit=100")

		require.Equal(t, 200, status)
		titles := asc(env)

		assert.Equal(t, []string{"c09", "c02", "c05", "c12", "c01", "c10", "c03", "c08"}, titles[:withDates])
		assert.ElementsMatch(t, []string{"c04", "c06", "c07", "c11"}, titles[withDates:])
	})

	t.Run("a ordenação por data funciona com paginação e o total é de todos", func(t *testing.T) {
		_, first := f.query(t, f.admin.Token, "sort=due_date&limit=5&page=1")
		_, second := f.query(t, f.admin.Token, "sort=due_date&limit=5&page=2")
		_, third := f.query(t, f.admin.Token, "sort=due_date&limit=5&page=3")

		assert.Equal(t, []string{"c08", "c03", "c10", "c01", "c12"}, shortTitles(first))
		assert.Equal(t, []string{"c05", "c02", "c09"}, shortTitles(second)[:3])
		assert.Len(t, third.List(), 2)
		assert.Equal(t, float64(12), first.Pagination()["total"])
		assert.Equal(t, float64(3), first.Pagination()["total_pages"])
	})
}

func TestCardFilters_AcceptanceCriteria(t *testing.T) {
	f := newFilterFixture(t)

	t.Run("meus cards urgentes abertos, do que vence antes ao que vence depois", func(t *testing.T) {
		status, env := f.query(t, f.member.Token, "priority=urgent&assignee_id=me&status=open&sort=due_date")

		require.Equal(t, 200, status)
		assert.Equal(t, []string{"c01", "c12"}, shortTitles(env))
	})

	t.Run("filtros com busca e paginação", func(t *testing.T) {
		status, env := f.query(t, f.admin.Token, "q=login&priority=urgent,high&sort=title&limit=2&page=2")

		require.Equal(t, 200, status)
		// c01, c02 (urgent), c03, c04 (high) têm "login": 4 itens, 2 por página.
		assert.Equal(t, []string{"c03", "c04"}, shortTitles(env))
		assert.Equal(t, float64(4), env.Pagination()["total"])
		assert.Equal(t, float64(2), env.Pagination()["total_pages"])
		assert.Equal(t, false, env.Pagination()["has_next"])
	})

	t.Run("sem filtros continua agrupado por coluna", func(t *testing.T) {
		status, env := f.query(t, f.admin.Token, "limit=100")

		require.Equal(t, 200, status)
		assert.Contains(t, string(env.Data), `"columns"`)
	})

	errorCases := []struct {
		name  string
		query string
		want  string
	}{
		{"prioridade inválida", "priority=xyz", "Invalid priority. Allowed: low, medium, high, urgent"},
		{"sort fora da allowlist", "sort=password", "Invalid sort. Allowed: position, created_at, updated_at, due_date, priority, title"},
		{"label_ids inválido", "label_ids=nao-e-id", `Invalid label_ids: "nao-e-id" is not a valid id`},
		{"assignee_id inválido", "assignee_id=xyz", "Invalid assignee_id. Use an id, me or none"},
		{"data inválida", "due_before=amanha", "Invalid due_date. Use RFC 3339 (2026-12-01T18:00:00Z) or YYYY-MM-DD, between 2000 and 10 years ahead"},
		{"due_after depois de due_before", "due_after=2026-12-10&due_before=2026-12-01", "due_after must not be after due_before"},
		{"overdue com completed", "overdue=true&status=completed", "overdue=true cannot be combined with status=completed"},
		{"q curto", "q=a", "Search term must have between 2 and 100 characters"},
		{"paginação inválida", "priority=high&page=abc", "Invalid pagination params"},
		{"coluna inexistente", "column_id=nao-existe", "Column not found in this board"},
	}

	for _, tc := range errorCases {
		t.Run("400: "+tc.name, func(t *testing.T) {
			status, env := f.query(t, f.admin.Token, tc.query)

			assert.Equal(t, 400, status)
			assert.Equal(t, tc.want, *env.Error)
		})
	}

	t.Run("a permissão vem antes dos filtros: não-membro recebe 404", func(t *testing.T) {
		status, _ := f.query(t, f.outsider.Token, "priority=high")

		assert.Equal(t, 404, status)
	})

	t.Run("os filtros não vazam cards deletados", func(t *testing.T) {
		_, env := f.query(t, f.admin.Token, "q=c09")
		require.Len(t, env.List(), 1)

		status, _ := f.do(t, "DELETE", "/card/"+env.List()[0]["id"].(string), f.admin.Token, nil)
		require.Equal(t, 200, status)

		_, env = f.query(t, f.admin.Token, "priority=high&column_id=doing")
		assert.Equal(t, []string{"c04"}, shortTitles(env))
	})
}

func TestProjectSearch_AcceptsTheSameFilters(t *testing.T) {
	f := newFilterFixture(t)
	path := "/card/project/" + f.projectID + "/search?q=login&"

	status, env := f.do(t, "GET", path+"priority=urgent", f.member.Token, nil)

	require.Equal(t, 200, status)
	assert.ElementsMatch(t, []string{"c01", "c02"}, shortTitles(env))

	status, env = f.do(t, "GET", path+"status=open&sort=title&order=desc&assignee_id=me", f.member.Token, nil)

	require.Equal(t, 200, status)
	assert.Equal(t, []string{"c01"}, shortTitles(env))

	status, _ = f.do(t, "GET", path+"priority=xyz", f.member.Token, nil)
	assert.Equal(t, 400, status)

	status, _ = f.do(t, "GET", "/card/project/"+f.projectID+"/search?priority=high", f.member.Token, nil)
	assert.Equal(t, 400, status, "q continua obrigatório na busca do projeto")
}

func TestCardFilters_QueryStringIsEscaped(t *testing.T) {
	f := newFilterFixture(t)

	status, env := f.query(t, f.admin.Token, "q="+url.QueryEscape("(a+)+$"))

	require.Equal(t, 200, status)
	assert.Empty(t, env.List())
}
