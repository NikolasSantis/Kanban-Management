package integration_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (s cardScenario) createCardWith(t *testing.T, column, title, description string) string {
	t.Helper()

	status, env := s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{
		"title": title, "description": description, "column_id": column,
	})
	require.Equal(t, 201, status, "create card %q: %v", title, env.Error)

	return env.Map()["id"].(string)
}

func (s cardScenario) searchCards(t *testing.T, token, q string) (int, testutil.Envelope) {
	t.Helper()

	return s.do(t, "GET", s.cardsPath()+"?q="+url.QueryEscape(q), token, nil)
}

func titlesOf(env testutil.Envelope) []string {
	titles := []string{}

	for _, item := range env.List() {
		titles = append(titles, item["title"].(string))
	}

	return titles
}

func TestSearchCards_OnBoard(t *testing.T) {
	s := newCardScenario(t)
	s.createCardWith(t, "todo", "Implementação de Login", "")
	s.createCardWith(t, "doing", "Autenticação JWT", "usar tokens de curta duração")
	s.createCardWith(t, "todo", "Outra coisa", "sem relação")

	cases := []struct {
		name string
		q    string
		want []string
	}{
		{"parte da palavra", "impl", []string{"Implementação de Login"}},
		{"caixa alta", "LOGIN", []string{"Implementação de Login"}},
		{"várias palavras, em qualquer ordem", "login impl", []string{"Implementação de Login"}},
		{"com acento no termo", "implementação", []string{"Implementação de Login"}},
		{"sem acento no termo, acento no título", "autenticacao", []string{"Autenticação JWT"}},
		{"acento no termo, sem acento no texto", "autenticação", []string{"Autenticação JWT"}},
		{"busca na descrição", "curta duracao", []string{"Autenticação JWT"}},
		{"não encontra o que não existe", "logout", []string{}},
		{"todas as palavras precisam aparecer", "login jwt", []string{}},
		{"regex vira texto literal: .* não retorna tudo", ".*", []string{}},
		{"regex catastrófica é tratada como texto", "(a+)+$", []string{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			status, env := s.searchCards(t, s.member.Token, tc.q)

			require.Equal(t, 200, status)
			assert.ElementsMatch(t, tc.want, titlesOf(env))
			assert.Less(t, time.Since(start), 5*time.Second, "sem lentidão")
		})
	}

	t.Run("a resposta é uma lista plana paginada, não agrupada por coluna", func(t *testing.T) {
		status, env := s.searchCards(t, s.admin.Token, "ao")

		require.Equal(t, 200, status)
		assert.NotNil(t, env.Pagination())
		assert.Len(t, env.List(), 3, "cards de colunas diferentes na mesma lista")
		assert.NotNil(t, env.List()[0]["column_id"])
		assert.NotContains(t, string(env.Data), `"columns"`)
	})

	t.Run("?column_id= restringe a busca a uma coluna", func(t *testing.T) {
		status, env := s.do(t, "GET", s.cardsPath()+"?q=ao&column_id=doing", s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, []string{"Autenticação JWT"}, titlesOf(env))

		status, _ = s.do(t, "GET", s.cardsPath()+"?q=ao&column_id=nao-existe", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("q vazio ou só espaços é a listagem normal", func(t *testing.T) {
		for _, q := range []string{"", "   "} {
			status, env := s.do(t, "GET", s.cardsPath()+"?q="+url.QueryEscape(q), s.admin.Token, nil)

			require.Equal(t, 200, status)
			assert.Contains(t, string(env.Data), `"columns"`, "listagem agrupada por coluna")
		}
	})

	t.Run("q inválido devolve 400 com a mensagem", func(t *testing.T) {
		for _, q := range []string{"a", " a ", strings.Repeat("a", 101)} {
			status, env := s.searchCards(t, s.admin.Token, q)

			assert.Equal(t, 400, status, "q=%q", q)
			assert.Equal(t, "Search term must have between 2 and 100 characters", *env.Error)
		}
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.searchCards(t, s.outsider.Token, "login")
		assert.Equal(t, 404, status)
	})
}

func TestSearchCards_FollowsEditsAndDeletes(t *testing.T) {
	s := newCardScenario(t)
	cardID := s.createCardWith(t, "todo", "Implementar Login", "primeira versão")

	found := func(q string) bool {
		_, env := s.searchCards(t, s.admin.Token, q)

		return len(env.List()) == 1
	}

	require.True(t, found("login"))

	t.Run("renomear o título atualiza a busca", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"title": "Fluxo de Logout"})
		require.Equal(t, 200, status)

		assert.False(t, found("login"))
		assert.True(t, found("logout"))
		assert.True(t, found("primeira"), "a descrição que não mudou continua indexada")
	})

	t.Run("mudar só a descrição mantém o título indexado", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"description": "segunda revisão"})
		require.Equal(t, 200, status)

		assert.False(t, found("primeira"))
		assert.True(t, found("revisao"))
		assert.True(t, found("logout"))
	})

	t.Run("search_text enviado pelo cliente é ignorado", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", "/card/"+cardID, s.admin.Token, map[string]any{"search_text": "forjado"})
		assert.Equal(t, 400, status, "campo fora da allowlist")

		status, _ = s.do(t, "POST", s.cardsPath(), s.admin.Token, map[string]any{
			"title": "Verdadeiro", "column_id": "todo", "search_text": "forjado",
		})
		require.Equal(t, 201, status)

		assert.False(t, found("forjado"))
	})

	t.Run("card deletado não aparece", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/card/"+cardID, s.admin.Token, nil)
		require.Equal(t, 200, status)

		assert.False(t, found("logout"))
	})
}

func TestSearchCards_ResultsAreOrderedByUpdatedAtDesc(t *testing.T) {
	s := newCardScenario(t)
	first := s.createCardWith(t, "todo", "tarefa alfa", "")
	s.createCardWith(t, "todo", "tarefa beta", "")

	_, env := s.searchCards(t, s.admin.Token, "tarefa")
	assert.Equal(t, []string{"tarefa beta", "tarefa alfa"}, titlesOf(env))

	status, _ := s.do(t, "PATCH", "/card/"+first, s.admin.Token, map[string]any{"description": "mexi aqui"})
	require.Equal(t, 200, status)

	_, env = s.searchCards(t, s.admin.Token, "tarefa")
	assert.Equal(t, []string{"tarefa alfa", "tarefa beta"}, titlesOf(env), "o último alterado vem primeiro")
}

func TestSearchCards_IsPaginated(t *testing.T) {
	s := newCardScenario(t)

	for i := 0; i < 5; i++ {
		s.createCardWith(t, "todo", "item de busca", "")
	}

	_, env := s.do(t, "GET", s.cardsPath()+"?q=busca&limit=2&page=3", s.admin.Token, nil)

	assert.Len(t, env.List(), 1)
	assert.Equal(t, float64(5), env.Pagination()["total"])
	assert.Equal(t, float64(3), env.Pagination()["total_pages"])
}

func TestSearchProjectCards(t *testing.T) {
	s := newCardScenario(t)
	s.createCardWith(t, "todo", "Implementar Login", "")

	secondBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "backlog")
	secondCard := testutil.CreateCard(t, s.app, s.admin.Token, secondBoard, "backlog", "Revisar login do admin")

	// Outro projeto do mesmo admin: não pode aparecer na busca deste.
	otherProject := testutil.CreateProject(t, s.app, s.admin.Token, s.workspaceID)
	otherBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, otherProject, "todo")
	testutil.CreateCard(t, s.app, s.admin.Token, otherBoard, "todo", "Login em outro projeto")

	deleted := testutil.CreateCard(t, s.app, s.admin.Token, secondBoard, "backlog", "Login apagado")
	status, _ := s.do(t, "DELETE", "/card/"+deleted, s.admin.Token, nil)
	require.Equal(t, 200, status)

	path := "/card/project/" + s.projectID + "/search"

	t.Run("acha cards de todos os boards do projeto, com board_id e column_id", func(t *testing.T) {
		status, env := s.do(t, "GET", path+"?q=login", s.member.Token, nil)

		require.Equal(t, 200, status)
		assert.ElementsMatch(t, []string{"Implementar Login", "Revisar login do admin"}, titlesOf(env))
		assert.NotNil(t, env.Pagination())

		for _, item := range env.List() {
			assert.NotEmpty(t, item["board_id"])
			assert.NotEmpty(t, item["column_id"])

			if item["id"] == secondCard {
				assert.Equal(t, secondBoard, item["board_id"])
				assert.Equal(t, "backlog", item["column_id"])
			}
		}
	})

	cases := []struct {
		name       string
		path       string
		token      string
		wantStatus int
	}{
		{"q é obrigatório", path, s.admin.Token, 400},
		{"q muito curto", path + "?q=a", s.admin.Token, 400},
		{"paginação inválida", path + "?q=login&page=abc", s.admin.Token, 400},
		{"não-membro recebe 404", path + "?q=login", s.outsider.Token, 404},
		{"sem autenticação", path + "?q=login", "", 401},
		{"id de projeto inválido", "/card/project/xyz/search?q=login", s.admin.Token, 400},
		{"projeto inexistente", "/card/project/000000000000000000000001/search?q=login", s.admin.Token, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "GET", tc.path, tc.token, nil)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestSearchBoardsByName(t *testing.T) {
	s := newBoardScenario(t)

	for _, name := range []string{"Sprint de Implementação", "Roadmap", "Backlog"} {
		status, env := s.do(t, "POST", "/board/project/"+s.projectID, s.admin.Token, map[string]any{"name": name, "search_text": "forjado"})
		require.Equal(t, 201, status, "%v", env.Error)
	}

	names := func(q string) []string {
		status, env := s.do(t, "GET", "/board/project/"+s.projectID+"?q="+url.QueryEscape(q), s.member.Token, nil)
		require.Equal(t, 200, status)

		out := []string{}
		for _, board := range env.List() {
			out = append(out, board["name"].(string))
		}

		return out
	}

	assert.Equal(t, []string{"Sprint de Implementação"}, names("implementacao"))
	assert.Equal(t, []string{"Roadmap"}, names("ROAD"))
	assert.Empty(t, names("forjado"), "search_text do cliente é ignorado")
	assert.Empty(t, names(".*"))
	assert.Len(t, names(""), 3, "sem q é a listagem normal")

	t.Run("renomear atualiza a busca; search_text no PATCH é protegido", func(t *testing.T) {
		_, env := s.do(t, "GET", "/board/project/"+s.projectID+"?q=roadmap", s.admin.Token, nil)
		boardID := env.List()[0]["id"].(string)

		status, _ := s.do(t, "PATCH", "/board/"+boardID, s.admin.Token, map[string]any{"search_text": "forjado"})
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "PATCH", "/board/"+boardID, s.admin.Token, map[string]any{"name": "Planejamento"})
		require.Equal(t, 200, status)

		assert.Empty(t, names("roadmap"))
		assert.Equal(t, []string{"Planejamento"}, names("planej"))
	})

	t.Run("q inválido e não-membro", func(t *testing.T) {
		status, _ := s.do(t, "GET", "/board/project/"+s.projectID+"?q=a", s.admin.Token, nil)
		assert.Equal(t, 400, status)

		status, _ = s.do(t, "GET", "/board/project/"+s.projectID+"?q=road", s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})
}

func TestSearchProjectsByNameAndDescription(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)

	for _, p := range []map[string]any{
		{"name": "Aplicativo Móvel", "description": "versão para celular"},
		{"name": "Site", "description": "página institucional"},
	} {
		p["workspace_id"] = workspaceID
		status, env := testutil.Do(t, app, "POST", "/project/", owner.Token, p)
		require.Equal(t, 201, status, "%v", env.Error)
	}

	names := func(q string) []string {
		status, env := testutil.Do(t, app, "GET", "/project/?q="+url.QueryEscape(q), owner.Token, nil)
		require.Equal(t, 200, status)

		out := []string{}
		for _, project := range env.List() {
			out = append(out, project["name"].(string))
		}

		return out
	}

	assert.Equal(t, []string{"Aplicativo Móvel"}, names("movel"), "pelo nome, sem acento")
	assert.Equal(t, []string{"Site"}, names("institucional"), "pela descrição")
	assert.Equal(t, []string{"Aplicativo Móvel"}, names("celular aplic"))
	assert.Empty(t, names("inexistente"))
	assert.Empty(t, names(".*"))
	assert.Len(t, names(""), 2)

	status, _ := testutil.Do(t, app, "GET", "/project/?q=a", owner.Token, nil)
	assert.Equal(t, 400, status)

	t.Run("editar nome e descrição atualiza a busca", func(t *testing.T) {
		_, env := testutil.Do(t, app, "GET", "/project/?q=site", owner.Token, nil)
		projectID := env.List()[0]["id"].(string)

		status, _ := testutil.Do(t, app, "PATCH", "/project/"+projectID, owner.Token, map[string]any{"name": "Portal"})
		require.Equal(t, 200, status)

		assert.Empty(t, names("site"))
		assert.Equal(t, []string{"Portal"}, names("portal"))
		assert.Equal(t, []string{"Portal"}, names("institucional"), "a descrição que não mudou continua indexada")

		status, _ = testutil.Do(t, app, "PATCH", "/project/"+projectID, owner.Token, map[string]any{"description": "documentação"})
		require.Equal(t, 200, status)

		assert.Empty(t, names("institucional"))
		assert.Equal(t, []string{"Portal"}, names("documentacao"))
	})
}
