package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddMember_UserMustExistAndBeActive(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	deleted := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)
	projectID := testutil.CreateProject(t, app, owner.Token, workspaceID)

	status, _ := testutil.Do(t, app, "DELETE", "/users/"+deleted.ID, deleted.Token, nil)
	require.Equal(t, 200, status)

	cases := []struct {
		name   string
		userID string
	}{
		{"usuário inexistente", "000000000000000000000001"},
		{"usuário deletado", deleted.ID},
	}

	for _, tc := range cases {
		t.Run("projeto: "+tc.name, func(t *testing.T) {
			status, _ := testutil.Do(t, app, "POST", "/project/member/"+projectID, owner.Token, map[string]any{
				"user_id": tc.userID, "role": "member",
			})
			assert.Equal(t, 404, status)
		})

		t.Run("workspace: "+tc.name, func(t *testing.T) {
			status, _ := testutil.Do(t, app, "POST", "/workspace/member/"+workspaceID, owner.Token, map[string]any{
				"user_id": tc.userID, "role": "member",
			})
			assert.Equal(t, 404, status)
		})
	}
}
