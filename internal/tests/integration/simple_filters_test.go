package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterProjects_ByStatusAndWorkspace(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	workspaceA := testutil.CreateWorkspace(t, app, owner.Token)
	workspaceB := testutil.CreateWorkspace(t, app, owner.Token)

	activeA := testutil.CreateProject(t, app, owner.Token, workspaceA)
	archivedA := testutil.CreateProject(t, app, owner.Token, workspaceA)
	activeB := testutil.CreateProject(t, app, owner.Token, workspaceB)

	status, _ := testutil.Do(t, app, "PATCH", "/project/"+archivedA, owner.Token, map[string]any{"status": "archived"})
	require.Equal(t, 200, status)

	ids := func(query string) []string {
		status, env := testutil.Do(t, app, "GET", "/project/?"+query, owner.Token, nil)
		require.Equal(t, 200, status, query)

		out := []string{}
		for _, project := range env.List() {
			out = append(out, project["id"].(string))
		}

		return out
	}

	assert.ElementsMatch(t, []string{activeA, activeB}, ids("status=active"))
	assert.ElementsMatch(t, []string{archivedA}, ids("status=archived"))
	assert.ElementsMatch(t, []string{activeA, archivedA, activeB}, ids("status=active,archived"))
	assert.ElementsMatch(t, []string{activeA, archivedA}, ids("workspace_id="+workspaceA))
	assert.ElementsMatch(t, []string{activeB}, ids("workspace_id="+workspaceB))
	assert.ElementsMatch(t, []string{activeA}, ids("workspace_id="+workspaceA+"&status=active"))
	assert.Empty(t, ids("status=nao-existe"))

	status, env := testutil.Do(t, app, "GET", "/project/?workspace_id=xyz", owner.Token, nil)
	assert.Equal(t, 400, status)
	assert.Equal(t, "Invalid workspace_id", *env.Error)
}

func TestFilterMembers_ByRole(t *testing.T) {
	s := newBoardScenario(t) // projeto: admin, manager, member

	t.Run("membros do projeto", func(t *testing.T) {
		roles := func(query string) []string {
			status, env := s.do(t, "GET", "/project/member/"+s.projectID+"?"+query, s.admin.Token, nil)
			require.Equal(t, 200, status, query)

			out := []string{}
			for _, member := range env.List() {
				out = append(out, member["role"].(string))
			}

			return out
		}

		assert.Len(t, roles(""), 3)
		assert.Equal(t, []string{"admin"}, roles("role=admin"))
		assert.ElementsMatch(t, []string{"admin", "manager"}, roles("role=admin,manager"))
		assert.Equal(t, []string{"member"}, roles("role=member"))

		status, env := s.do(t, "GET", "/project/member/"+s.projectID+"?role=dono", s.admin.Token, nil)
		assert.Equal(t, 400, status)
		assert.Equal(t, "Invalid role. Allowed: member, manager, admin", *env.Error)
	})

	t.Run("membros do workspace", func(t *testing.T) {
		status, _ := s.do(t, "POST", "/workspace/member/"+s.workspaceID, s.admin.Token, map[string]any{"user_id": s.member.ID, "role": "member"})
		require.Equal(t, 200, status)

		status, env := s.do(t, "GET", "/workspace/member/"+s.workspaceID+"?role=admin", s.admin.Token, nil)
		require.Equal(t, 200, status)
		require.Len(t, env.List(), 1)
		assert.Equal(t, s.admin.ID, env.List()[0]["user_id"])

		status, env = s.do(t, "GET", "/workspace/member/"+s.workspaceID+"?role=member", s.admin.Token, nil)
		require.Equal(t, 200, status)
		require.Len(t, env.List(), 1)
		assert.Equal(t, s.member.ID, env.List()[0]["user_id"])

		status, _ = s.do(t, "GET", "/workspace/member/"+s.workspaceID+"?role=xyz", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})
}

func TestFilterComments_ByAuthor(t *testing.T) {
	s := newCommentScenario(t)
	s.comment(t, s.admin.Token, "do admin")
	s.comment(t, s.member.Token, "do member 1")
	s.comment(t, s.member.Token, "do member 2")

	contentsOf := func(token, query string) []string {
		status, env := s.do(t, "GET", s.commentsPath()+"?"+query, token, nil)
		require.Equal(t, 200, status, query)

		out := []string{}
		for _, comment := range env.List() {
			out = append(out, comment["content"].(string))
		}

		return out
	}

	assert.Equal(t, []string{"do member 1", "do member 2"}, contentsOf(s.admin.Token, "author_id="+s.member.ID))
	assert.Equal(t, []string{"do member 1", "do member 2"}, contentsOf(s.member.Token, "author_id=me"))
	assert.Equal(t, []string{"do admin"}, contentsOf(s.admin.Token, "author_id=me"))
	assert.Empty(t, contentsOf(s.admin.Token, "author_id="+s.manager.ID))

	status, env := s.do(t, "GET", s.commentsPath()+"?author_id=xyz", s.admin.Token, nil)
	assert.Equal(t, 400, status)
	assert.Equal(t, "Invalid author_id. Use an id or me", *env.Error)
}

func TestFilterAttachments_ByKind(t *testing.T) {
	s := newAttachmentScenario(t)
	s.uploadOK(t, s.admin.Token, "a.png", pngBytes)
	s.uploadOK(t, s.admin.Token, "b.png", pngBytes)
	status, _ := s.do(t, "POST", s.path()+"/link", s.admin.Token, map[string]any{"name": "doc", "url": "https://exemplo.com"})
	require.Equal(t, 201, status)

	kinds := func(query string) []string {
		status, env := s.do(t, "GET", s.path()+"?"+query, s.admin.Token, nil)
		require.Equal(t, 200, status, query)

		out := []string{}
		for _, attachment := range env.List() {
			out = append(out, attachment["kind"].(string))
		}

		return out
	}

	assert.Len(t, kinds(""), 3)
	assert.Equal(t, []string{"file", "file"}, kinds("kind=file"))
	assert.Equal(t, []string{"link"}, kinds("kind=link"))

	status, env := s.do(t, "GET", s.path()+"?kind=pdf", s.admin.Token, nil)
	assert.Equal(t, 400, status)
	assert.Equal(t, "Invalid kind. Allowed: file, link", *env.Error)
}
