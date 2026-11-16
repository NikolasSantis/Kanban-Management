package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allDeleted confere que o escopo tem registros e que TODOS têm deleted_at.
func allDeleted(t *testing.T, collection, field, id string, wantTotal int64) {
	t.Helper()

	total, deleted := membersState(t, collection, field, id)

	assert.Equal(t, wantTotal, total, "%s: os registros continuam no banco", collection)
	assert.Equal(t, total, deleted, "%s: todos devem ter deleted_at", collection)
}

// populate cria 2 labels e 2 cards no board do cenário.
func (s cardScenario) populate(t *testing.T) {
	t.Helper()

	s.newLabel(t, "Bug")
	s.newLabel(t, "Feature")
	testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "um")
	testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "dois")
}

func TestDeleteBoard_SoftDeletesItsLabels(t *testing.T) {
	s := newCardScenario(t)
	s.populate(t)

	status, _ := s.do(t, "DELETE", "/board/"+s.boardID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	allDeleted(t, "labels", "board_id", s.boardID, 2)
	allDeleted(t, "cards", "board_id", s.boardID, 2)
}

func TestDeleteProject_CascadesToBoardsCardsAndLabels(t *testing.T) {
	s := newCardScenario(t)
	s.populate(t)
	secondBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, s.projectID, "todo")
	testutil.CreateCard(t, s.app, s.admin.Token, secondBoard, "todo", "no-segundo")

	status, _ := s.do(t, "DELETE", "/project/"+s.projectID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	allDeleted(t, "boards", "project_id", s.projectID, 2)
	allDeleted(t, "cards", "project_id", s.projectID, 3)
	allDeleted(t, "labels", "project_id", s.projectID, 2)
	allDeleted(t, "project_members", "project_id", s.projectID, 3)

	status, _ = s.do(t, "GET", "/card/board/"+s.boardID, s.admin.Token, nil)
	assert.Equal(t, 404, status)
}

func TestDeleteProject_DoesNotTouchOtherProjects(t *testing.T) {
	s := newCardScenario(t)
	s.populate(t)

	otherProject := testutil.CreateProject(t, s.app, s.admin.Token, s.workspaceID)
	otherBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, otherProject, "todo")
	testutil.CreateCard(t, s.app, s.admin.Token, otherBoard, "todo", "fica")

	status, _ := s.do(t, "DELETE", "/project/"+s.projectID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	status, env := s.do(t, "GET", "/card/board/"+otherBoard+"?column_id=todo", s.admin.Token, nil)
	require.Equal(t, 200, status, "o outro projeto continua intacto")
	assert.Len(t, env.List(), 1)
}

func TestDeleteWorkspace_CascadesToProjectsAndEverythingInside(t *testing.T) {
	s := newCardScenario(t)
	s.populate(t)
	secondProject := testutil.CreateProject(t, s.app, s.admin.Token, s.workspaceID)
	secondBoard := testutil.CreateBoardWithColumns(t, s.app, s.admin.Token, secondProject, "todo")
	testutil.CreateCard(t, s.app, s.admin.Token, secondBoard, "todo", "no-outro-projeto")

	status, _ := s.do(t, "DELETE", "/workspace/"+s.workspaceID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	allDeleted(t, "projects", "workspace_id", s.workspaceID, 2)

	for _, projectID := range []string{s.projectID, secondProject} {
		total, deleted := membersState(t, "boards", "project_id", projectID)
		assert.Equal(t, total, deleted, "boards do projeto %s", projectID)

		total, deleted = membersState(t, "cards", "project_id", projectID)
		assert.Equal(t, total, deleted, "cards do projeto %s", projectID)

		total, deleted = membersState(t, "project_members", "project_id", projectID)
		assert.Equal(t, total, deleted, "membros do projeto %s", projectID)
	}

	allDeleted(t, "labels", "project_id", s.projectID, 2)
	allDeleted(t, "workspaces_members", "workspace_id", s.workspaceID, 1)

	_, env := s.do(t, "GET", "/project/", s.admin.Token, nil)
	assert.False(t, containsID(env.List(), s.projectID))
	assert.False(t, containsID(env.List(), secondProject))
}
