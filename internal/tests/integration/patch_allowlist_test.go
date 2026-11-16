package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchProject_OnlyAllowedFieldsAreUpdated(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	other := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)
	otherWorkspaceID := testutil.CreateWorkspace(t, app, other.Token)
	projectID := testutil.CreateProject(t, app, owner.Token, workspaceID)

	status, _ := testutil.Do(t, app, "PATCH", "/project/"+projectID, owner.Token, map[string]any{
		"name":         "Renomeado",
		"description":  "nova descrição",
		"status":       "archived",
		"owner_id":     other.ID,
		"workspace_id": otherWorkspaceID,
		"deleted_at":   "2020-01-01T00:00:00Z",
		"created_at":   "2001-01-01T00:00:00Z",
	})
	require.Equal(t, 200, status)

	_, env := testutil.Do(t, app, "GET", "/project/", owner.Token, nil)

	var project map[string]any
	for _, p := range env.List() {
		if p["id"] == projectID {
			project = p
		}
	}
	require.NotNil(t, project, "o projeto não pode ter sido deletado nem trocado de dono")

	assert.Equal(t, "Renomeado", project["name"])
	assert.Equal(t, "nova descrição", project["description"])
	assert.Equal(t, "archived", project["status"])
	assert.Equal(t, owner.ID, project["owner_id"])
	assert.Equal(t, workspaceID, project["workspace_id"])
	assert.NotEqual(t, "2001-01-01T00:00:00Z", project["created_at"])

	t.Run("só campos protegidos devolve 400", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "PATCH", "/project/"+projectID, owner.Token, map[string]any{"owner_id": other.ID})
		assert.Equal(t, 400, status)
	})
}

func TestPatchWorkspace_OnlyAllowedFieldsAreUpdated(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	other := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)

	status, _ := testutil.Do(t, app, "PATCH", "/workspace/"+workspaceID, owner.Token, map[string]any{
		"name":        "Renomeado",
		"description": "nova descrição",
		"owner_id":    other.ID,
		"deleted_at":  "2020-01-01T00:00:00Z",
		"created_at":  "2001-01-01T00:00:00Z",
	})
	require.Equal(t, 200, status)

	_, env := testutil.Do(t, app, "GET", "/workspace/", owner.Token, nil)

	var workspace map[string]any
	for _, w := range env.List() {
		if w["id"] == workspaceID {
			workspace = w
		}
	}
	require.NotNil(t, workspace, "o workspace não pode ter sido deletado nem trocado de dono")

	assert.Equal(t, "Renomeado", workspace["name"])
	assert.Equal(t, "nova descrição", workspace["description"])
	assert.Equal(t, owner.ID, workspace["owner_id"])
	assert.NotEqual(t, "2001-01-01T00:00:00Z", workspace["created_at"])

	t.Run("só campos protegidos devolve 400", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "PATCH", "/workspace/"+workspaceID, owner.Token, map[string]any{"owner_id": other.ID})
		assert.Equal(t, 400, status)
	})
}
