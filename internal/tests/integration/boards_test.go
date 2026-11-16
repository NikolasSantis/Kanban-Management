package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boardScenario monta: admin (dono do projeto), manager, member e outsider (não-membro).
type boardScenario struct {
	app         *fiber.App
	admin       testutil.User
	manager     testutil.User
	member      testutil.User
	outsider    testutil.User
	workspaceID string
	projectID   string
}

func newBoardScenario(t *testing.T) boardScenario {
	t.Helper()

	app := testutil.Setup(t)

	s := boardScenario{
		app:      app,
		admin:    testutil.CreateUser(t, app),
		manager:  testutil.CreateUser(t, app),
		member:   testutil.CreateUser(t, app),
		outsider: testutil.CreateUser(t, app),
	}

	workspaceID := testutil.CreateWorkspace(t, app, s.admin.Token)
	s.workspaceID = workspaceID
	s.projectID = testutil.CreateProject(t, app, s.admin.Token, workspaceID)

	testutil.AddProjectMember(t, app, s.admin.Token, s.projectID, s.manager.ID, "manager")
	testutil.AddProjectMember(t, app, s.admin.Token, s.projectID, s.member.ID, "member")

	return s
}

func (s boardScenario) do(t *testing.T, method, path, token string, body any) (int, testutil.Envelope) {
	t.Helper()
	return testutil.Do(t, s.app, method, path, token, body)
}

func TestGetBoards(t *testing.T) {
	s := newBoardScenario(t)
	testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

	cases := []struct {
		name       string
		path       string
		token      string
		wantStatus int
	}{
		{"admin lista", "/board/project/" + s.projectID, s.admin.Token, 200},
		{"member lista", "/board/project/" + s.projectID, s.member.Token, 200},
		{"não-membro recebe 404", "/board/project/" + s.projectID, s.outsider.Token, 404},
		{"id de projeto inválido", "/board/project/id-invalido", s.admin.Token, 400},
		{"projeto inexistente", "/board/project/000000000000000000000001", s.admin.Token, 404},
		{"sem autenticação", "/board/project/" + s.projectID, "", 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, env := s.do(t, "GET", tc.path, tc.token, nil)

			assert.Equal(t, tc.wantStatus, status)

			if tc.wantStatus == 200 {
				assert.Len(t, env.List(), 1)
			}
		})
	}
}

func TestCreateBoard(t *testing.T) {
	s := newBoardScenario(t)

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"admin cria", "/board/project/" + s.projectID, s.admin.Token, map[string]any{"name": "Quadro"}, 201},
		{"manager cria", "/board/project/" + s.projectID, s.manager.Token, map[string]any{"name": "Quadro"}, 201},
		{"member não cria", "/board/project/" + s.projectID, s.member.Token, map[string]any{"name": "Quadro"}, 403},
		{"não-membro recebe 404", "/board/project/" + s.projectID, s.outsider.Token, map[string]any{"name": "Quadro"}, 404},
		{"sem nome", "/board/project/" + s.projectID, s.admin.Token, map[string]any{"description": "x"}, 400},
		{"nome vazio", "/board/project/" + s.projectID, s.admin.Token, map[string]any{"name": ""}, 400},
		{"corpo inválido", "/board/project/" + s.projectID, s.admin.Token, "texto", 400},
		{"id de projeto inválido", "/board/project/xyz", s.admin.Token, map[string]any{"name": "Quadro"}, 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestCreateBoard_SetsProjectAndIgnoresClientIDs(t *testing.T) {
	s := newBoardScenario(t)

	status, env := s.do(t, "POST", "/board/project/"+s.projectID, s.admin.Token, map[string]any{
		"name":       "Quadro",
		"project_id": "000000000000000000000001",
		"deleted_at": "2020-01-01T00:00:00Z",
		"columns":    []map[string]any{{"name": "To do"}},
	})

	require.Equal(t, 201, status)
	board := env.Map()
	assert.Equal(t, s.projectID, board["project_id"], "project_id deve vir da rota, não do body")
	assert.Nil(t, board["deleted_at"])
	assert.NotEmpty(t, board["id"])
}

func TestGetBoard(t *testing.T) {
	s := newBoardScenario(t)
	boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

	cases := []struct {
		name       string
		boardID    string
		token      string
		wantStatus int
	}{
		{"admin vê", boardID, s.admin.Token, 200},
		{"member vê", boardID, s.member.Token, 200},
		{"não-membro recebe 404", boardID, s.outsider.Token, 404},
		{"id inválido", "id-invalido", s.admin.Token, 400},
		{"board inexistente", "000000000000000000000001", s.admin.Token, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, env := s.do(t, "GET", "/board/"+tc.boardID, tc.token, nil)

			assert.Equal(t, tc.wantStatus, status)

			if tc.wantStatus == 200 {
				assert.Equal(t, boardID, env.Map()["id"])
			}
		})
	}
}

func TestPatchBoard(t *testing.T) {
	s := newBoardScenario(t)
	boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

	cases := []struct {
		name       string
		boardID    string
		token      string
		body       any
		wantStatus int
	}{
		{"admin edita", boardID, s.admin.Token, map[string]any{"description": "nova"}, 200},
		{"manager edita", boardID, s.manager.Token, map[string]any{"color": "#fff"}, 200},
		{"member não edita", boardID, s.member.Token, map[string]any{"description": "x"}, 403},
		{"não-membro recebe 404", boardID, s.outsider.Token, map[string]any{"description": "x"}, 404},
		{"só campos protegidos", boardID, s.admin.Token, map[string]any{"project_id": "000000000000000000000001"}, 400},
		{"corpo vazio", boardID, s.admin.Token, map[string]any{}, 400},
		{"JSON inválido", boardID, s.admin.Token, "texto", 400},
		{"id inválido", "id-invalido", s.admin.Token, map[string]any{"description": "x"}, 400},
		{"board inexistente", "000000000000000000000001", s.admin.Token, map[string]any{"description": "x"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "PATCH", "/board/"+tc.boardID, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestPatchBoard_ProtectedFieldsAreIgnored(t *testing.T) {
	s := newBoardScenario(t)
	boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

	status, _ := s.do(t, "PATCH", "/board/"+boardID, s.admin.Token, map[string]any{
		"name":       "Renomeado",
		"project_id": "000000000000000000000001",
		"deleted_at": "2020-01-01T00:00:00Z",
		"created_at": "2001-01-01T00:00:00Z",
	})
	require.Equal(t, 200, status)

	status, env := s.do(t, "GET", "/board/"+boardID, s.admin.Token, nil)
	require.Equal(t, 200, status, "o board não pode ter sido deletado nem movido de projeto")

	board := env.Map()
	assert.Equal(t, "Renomeado", board["name"])
	assert.Equal(t, s.projectID, board["project_id"])
	assert.NotEqual(t, "2001-01-01T00:00:00Z", board["created_at"])
}

func TestDeleteBoard(t *testing.T) {
	s := newBoardScenario(t)

	t.Run("member não deleta", func(t *testing.T) {
		boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

		status, _ := s.do(t, "DELETE", "/board/"+boardID, s.member.Token, nil)

		assert.Equal(t, 403, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

		status, _ := s.do(t, "DELETE", "/board/"+boardID, s.outsider.Token, nil)

		assert.Equal(t, 404, status)
	})

	t.Run("id inválido", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", "/board/id-invalido", s.admin.Token, nil)

		assert.Equal(t, 400, status)
	})

	t.Run("admin deleta (soft delete)", func(t *testing.T) {
		boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

		status, _ := s.do(t, "DELETE", "/board/"+boardID, s.admin.Token, nil)
		require.Equal(t, 200, status)

		status, _ = s.do(t, "GET", "/board/"+boardID, s.admin.Token, nil)
		assert.Equal(t, 404, status, "board deletado não pode ser lido")

		_, env := s.do(t, "GET", "/board/project/"+s.projectID, s.admin.Token, nil)
		for _, b := range env.List() {
			assert.NotEqual(t, boardID, b["id"], "board deletado não pode aparecer na listagem")
		}

		status, _ = s.do(t, "PATCH", "/board/"+boardID, s.admin.Token, map[string]any{"name": "x"})
		assert.Equal(t, 404, status, "board deletado não pode ser editado")

		status, _ = s.do(t, "DELETE", "/board/"+boardID, s.admin.Token, nil)
		assert.Equal(t, 404, status, "deletar duas vezes devolve 404")
	})

	t.Run("manager deleta", func(t *testing.T) {
		boardID := testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)

		status, _ := s.do(t, "DELETE", "/board/"+boardID, s.manager.Token, nil)

		assert.Equal(t, 200, status)
	})
}
