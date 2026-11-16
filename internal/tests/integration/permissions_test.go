package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchUser_OtherUser_ReturnsForbidden(t *testing.T) {
	app := testutil.Setup(t)
	a := testutil.CreateUser(t, app)
	b := testutil.CreateUser(t, app)

	status, _ := testutil.Do(t, app, "PATCH", "/users/"+a.ID, b.Token, map[string]any{"name": "sequestrado"})

	assert.Equal(t, 403, status)

	_, env := testutil.Do(t, app, "GET", "/auth/me", a.Token, nil)
	assert.NotEqual(t, "sequestrado", env.Map()["name"], "o nome do outro usuário não pode ter mudado")
}

func TestPatchUser_Self(t *testing.T) {
	app := testutil.Setup(t)
	a := testutil.CreateUser(t, app)
	other := testutil.CreateUser(t, app)

	cases := []struct {
		name       string
		body       map[string]any
		wantStatus int
	}{
		{"edita o próprio nome", map[string]any{"name": "Novo Nome"}, 200},
		{"e-mail inválido", map[string]any{"email": "nao-e-email"}, 400},
		{"e-mail com tipo errado", map[string]any{"email": 123}, 400},
		{"nome vazio", map[string]any{"name": "  "}, 400},
		{"e-mail já usado por outro", map[string]any{"email": other.Email}, 409},
		{"sem campos válidos", map[string]any{"password": "x"}, 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := testutil.Do(t, app, "PATCH", "/users/"+a.ID, a.Token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestCreateProject_WorkspaceRules(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	stranger := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)

	cases := []struct {
		name        string
		token       string
		workspaceID string
		wantStatus  int
	}{
		{"membro do workspace cria", owner.Token, workspaceID, 201},
		{"workspace inexistente", owner.Token, "000000000000000000000001", 404},
		{"usuário que não é membro do workspace", stranger.Token, workspaceID, 404},
		{"id de workspace inválido", owner.Token, "invalido", 400},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := testutil.Do(t, app, "POST", "/project/", tc.token, map[string]any{
				"workspace_id": tc.workspaceID, "name": "p-" + testutil.RandomString(3),
			})
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestProjectMembers_RolesAndLastAdmin(t *testing.T) {
	s := newBoardScenario(t) // admin, manager, member e outsider
	base := "/project/member/" + s.projectID

	t.Run("member não adiciona membros", func(t *testing.T) {
		status, _ := s.do(t, "POST", base, s.member.Token, map[string]any{"user_id": s.outsider.ID, "role": "member"})
		assert.Equal(t, 403, status)
	})

	t.Run("não-membro recebe 404 ao ver membros", func(t *testing.T) {
		status, _ := s.do(t, "GET", base, s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("member vê os membros", func(t *testing.T) {
		status, env := s.do(t, "GET", base, s.member.Token, nil)
		assert.Equal(t, 200, status)
		assert.Len(t, env.List(), 3)
	})

	t.Run("manager não promove ninguém a admin", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", base, s.manager.Token, map[string]any{"user_id": s.member.ID, "role": "admin"})
		assert.Equal(t, 403, status)
	})

	t.Run("manager não adiciona admin", func(t *testing.T) {
		status, _ := s.do(t, "POST", base, s.manager.Token, map[string]any{"user_id": s.outsider.ID, "role": "admin"})
		assert.Equal(t, 403, status)
	})

	t.Run("manager não rebaixa nem remove admin", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", base, s.manager.Token, map[string]any{"user_id": s.admin.ID, "role": "member"})
		assert.Equal(t, 403, status)

		status, _ = s.do(t, "DELETE", base, s.manager.Token, map[string]any{"user_id": s.admin.ID})
		assert.Equal(t, 403, status)
	})

	t.Run("único admin não pode ser rebaixado nem removido", func(t *testing.T) {
		status, env := s.do(t, "PATCH", base, s.admin.Token, map[string]any{"user_id": s.admin.ID, "role": "member"})
		assert.Equal(t, 409, status)
		assert.Equal(t, "Project must have at least one admin", *env.Error)

		status, _ = s.do(t, "DELETE", base, s.admin.Token, map[string]any{"user_id": s.admin.ID})
		assert.Equal(t, 409, status)
	})

	t.Run("role inválida", func(t *testing.T) {
		status, _ := s.do(t, "POST", base, s.admin.Token, map[string]any{"user_id": s.outsider.ID, "role": "dono"})
		assert.Equal(t, 400, status)
	})

	t.Run("com dois admins, um deles pode sair", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", base, s.admin.Token, map[string]any{"user_id": s.manager.ID, "role": "admin"})
		require.Equal(t, 200, status)

		status, _ = s.do(t, "PATCH", base, s.admin.Token, map[string]any{"user_id": s.admin.ID, "role": "member"})
		assert.Equal(t, 200, status)

		// Agora o ex-manager é o único admin de novo.
		status, _ = s.do(t, "DELETE", base, s.manager.Token, map[string]any{"user_id": s.manager.ID})
		assert.Equal(t, 409, status)
	})
}

func TestProjects_RolesOnPatchAndDelete(t *testing.T) {
	s := newBoardScenario(t)
	path := "/project/" + s.projectID

	t.Run("member não edita", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.member.Token, map[string]any{"description": "x"})
		assert.Equal(t, 403, status)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.outsider.Token, map[string]any{"description": "x"})
		assert.Equal(t, 404, status)
	})

	t.Run("manager edita mas não deleta", func(t *testing.T) {
		status, _ := s.do(t, "PATCH", path, s.manager.Token, map[string]any{"description": "do manager"})
		assert.Equal(t, 200, status)

		status, _ = s.do(t, "DELETE", path, s.manager.Token, nil)
		assert.Equal(t, 403, status)
	})

	t.Run("projeto aparece na lista de quem é membro, e não na de quem não é", func(t *testing.T) {
		_, env := s.do(t, "GET", "/project/", s.member.Token, nil)
		assert.True(t, containsID(env.List(), s.projectID), "member deve ver o projeto em GET /project")

		_, env = s.do(t, "GET", "/project/", s.outsider.Token, nil)
		assert.False(t, containsID(env.List(), s.projectID), "não-membro não pode ver o projeto")
	})

	t.Run("admin deleta e o projeto some (soft delete)", func(t *testing.T) {
		status, _ := s.do(t, "DELETE", path, s.admin.Token, nil)
		require.Equal(t, 200, status)

		_, env := s.do(t, "GET", "/project/", s.admin.Token, nil)
		assert.False(t, containsID(env.List(), s.projectID))

		status, _ = s.do(t, "GET", "/board/project/"+s.projectID, s.admin.Token, nil)
		assert.Equal(t, 404, status, "projeto deletado não pode ter boards listados")
	})
}

func TestWorkspaces_RolesAndLastAdmin(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	member := testutil.CreateUser(t, app)
	outsider := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)
	base := "/workspace/member/" + workspaceID

	status, _ := testutil.Do(t, app, "POST", base, owner.Token, map[string]any{"user_id": member.ID, "role": "member"})
	require.Equal(t, 200, status)

	t.Run("duplicado devolve 409", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "POST", base, owner.Token, map[string]any{"user_id": member.ID, "role": "member"})
		assert.Equal(t, 409, status)
	})

	t.Run("member vê os membros", func(t *testing.T) {
		status, env := testutil.Do(t, app, "GET", base, member.Token, nil)
		assert.Equal(t, 200, status)
		assert.Len(t, env.List(), 2)
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "GET", base, outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("member não edita nem deleta o workspace", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "PATCH", "/workspace/"+workspaceID, member.Token, map[string]any{"description": "x"})
		assert.Equal(t, 403, status)

		status, _ = testutil.Do(t, app, "DELETE", "/workspace/"+workspaceID, member.Token, nil)
		assert.Equal(t, 403, status)
	})

	t.Run("member não adiciona membros", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "POST", base, member.Token, map[string]any{"user_id": outsider.ID, "role": "member"})
		assert.Equal(t, 403, status)
	})

	t.Run("único admin não pode ser rebaixado nem removido", func(t *testing.T) {
		status, env := testutil.Do(t, app, "PATCH", base, owner.Token, map[string]any{"user_id": owner.ID, "role": "member"})
		assert.Equal(t, 409, status)
		assert.Equal(t, "Workspace must have at least one admin", *env.Error)

		status, _ = testutil.Do(t, app, "DELETE", base, owner.Token, map[string]any{"user_id": owner.ID})
		assert.Equal(t, 409, status)
	})

	t.Run("admin promove member a manager", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "PATCH", base, owner.Token, map[string]any{"user_id": member.ID, "role": "manager"})
		assert.Equal(t, 200, status)
	})

	t.Run("manager não promove a admin", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "PATCH", base, member.Token, map[string]any{"user_id": member.ID, "role": "admin"})
		assert.Equal(t, 403, status)
	})

	t.Run("admin deleta o workspace e ele some da lista", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "DELETE", "/workspace/"+workspaceID, owner.Token, nil)
		require.Equal(t, 200, status)

		_, env := testutil.Do(t, app, "GET", "/workspace/", owner.Token, nil)
		assert.False(t, containsID(env.List(), workspaceID))
	})
}

func containsID(items []map[string]any, id string) bool {
	for _, item := range items {
		if item["id"] == id {
			return true
		}
	}

	return false
}
