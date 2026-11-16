package services_test

import (
	"context"
	"testing"

	"kanban-management/internal/services"
	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Estes testes usam o banco de teste (testutil.Setup pula quando não há banco).

func oid(t *testing.T, hex string) bson.ObjectID {
	t.Helper()

	id, err := bson.ObjectIDFromHex(hex)
	require.NoError(t, err)

	return id
}

type fixture struct {
	admin, manager, member, outsider testutil.User
	workspaceID, projectID           bson.ObjectID
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	app := testutil.Setup(t)

	f := fixture{
		admin:    testutil.CreateUser(t, app),
		manager:  testutil.CreateUser(t, app),
		member:   testutil.CreateUser(t, app),
		outsider: testutil.CreateUser(t, app),
	}

	workspaceID := testutil.CreateWorkspace(t, app, f.admin.Token)
	projectID := testutil.CreateProject(t, app, f.admin.Token, workspaceID)

	testutil.AddProjectMember(t, app, f.admin.Token, projectID, f.manager.ID, "manager")
	testutil.AddProjectMember(t, app, f.admin.Token, projectID, f.member.ID, "member")

	f.workspaceID, f.projectID = oid(t, workspaceID), oid(t, projectID)

	return f
}

func TestGetProjectUserRole(t *testing.T) {
	f := newFixture(t)

	role, err := services.GetProjectUserRole(oid(t, f.admin.ID), f.projectID)
	require.NoError(t, err)
	assert.Equal(t, services.RoleAdmin, role)

	role, err = services.GetProjectUserRole(oid(t, f.member.ID), f.projectID)
	require.NoError(t, err)
	assert.Equal(t, services.RoleMember, role)

	_, err = services.GetProjectUserRole(oid(t, f.outsider.ID), f.projectID)
	assert.Error(t, err)
}

func TestAuthorizeProject(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		user     testutil.User
		action   services.Action
		wantRole string
		wantErr  error
	}{
		{"admin deleta projeto", f.admin, services.ActionProjectDelete, services.RoleAdmin, nil},
		{"manager edita projeto", f.manager, services.ActionProjectUpdate, services.RoleManager, nil},
		{"manager não deleta projeto", f.manager, services.ActionProjectDelete, services.RoleManager, services.ErrForbidden},
		{"member vê board", f.member, services.ActionBoardView, services.RoleMember, nil},
		{"member não gerencia board", f.member, services.ActionBoardManage, services.RoleMember, services.ErrForbidden},
		{"não-membro", f.outsider, services.ActionProjectView, "", services.ErrNotMember},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, err := services.AuthorizeProject(ctx, oid(t, tc.user.ID), f.projectID, tc.action)

			assert.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, tc.wantRole, role)
		})
	}

	t.Run("projeto inexistente", func(t *testing.T) {
		_, err := services.AuthorizeProject(ctx, oid(t, f.admin.ID), oid(t, "000000000000000000000001"), services.ActionProjectView)

		assert.ErrorIs(t, err, services.ErrNotMember)
	})
}

func TestAuthorizeWorkspace(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	role, err := services.AuthorizeWorkspace(ctx, oid(t, f.admin.ID), f.workspaceID, services.ActionWorkspaceDelete)
	require.NoError(t, err)
	assert.Equal(t, services.RoleAdmin, role, "o dono do workspace é admin")

	_, err = services.AuthorizeWorkspace(ctx, oid(t, f.outsider.ID), f.workspaceID, services.ActionWorkspaceView)
	assert.ErrorIs(t, err, services.ErrNotMember)

	_, err = services.AuthorizeWorkspace(ctx, oid(t, f.admin.ID), oid(t, "000000000000000000000001"), services.ActionWorkspaceView)
	assert.ErrorIs(t, err, services.ErrNotMember)

	app := testutil.Setup(t)
	status, _ := testutil.Do(t, app, "POST", "/workspace/member/"+f.workspaceID.Hex(), f.admin.Token,
		map[string]any{"user_id": f.member.ID, "role": "member"})
	require.Equal(t, 200, status)

	_, err = services.AuthorizeWorkspace(ctx, oid(t, f.member.ID), f.workspaceID, services.ActionWorkspaceUpdate)
	assert.ErrorIs(t, err, services.ErrForbidden)

	role, err = services.GetWorkspaceUserRole(oid(t, f.member.ID), f.workspaceID)
	require.NoError(t, err)
	assert.Equal(t, services.RoleMember, role)
}

func TestEnsureNotLastAdmin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("quem não é admin passa", func(t *testing.T) {
		assert.NoError(t, services.EnsureNotLastAdmin(ctx, "project_members", "project_id", f.projectID, services.RoleMember))
	})

	t.Run("único admin é barrado", func(t *testing.T) {
		err := services.EnsureNotLastAdmin(ctx, "project_members", "project_id", f.projectID, services.RoleAdmin)

		assert.ErrorIs(t, err, services.ErrLastAdmin)
	})

	t.Run("com dois admins passa", func(t *testing.T) {
		app := testutil.Setup(t)
		status, _ := testutil.Do(t, app, "PATCH", "/project/member/"+f.projectID.Hex(), f.admin.Token,
			map[string]any{"user_id": f.manager.ID, "role": "admin"})
		require.Equal(t, 200, status)

		assert.NoError(t, services.EnsureNotLastAdmin(ctx, "project_members", "project_id", f.projectID, services.RoleAdmin))
	})
}

func TestUserExists(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	assert.True(t, services.UserExists(ctx, oid(t, f.member.ID)))
	assert.False(t, services.UserExists(ctx, oid(t, "000000000000000000000001")))

	app := testutil.Setup(t)
	status, _ := testutil.Do(t, app, "DELETE", "/users/"+f.outsider.ID, f.outsider.Token, nil)
	require.Equal(t, 200, status)

	assert.False(t, services.UserExists(ctx, oid(t, f.outsider.ID)), "usuário deletado não existe")
}

func TestWorkspaceExists(t *testing.T) {
	f := newFixture(t)

	assert.True(t, services.WorkspaceExists(f.workspaceID))
	assert.False(t, services.WorkspaceExists(oid(t, "000000000000000000000001")))

	app := testutil.Setup(t)
	status, _ := testutil.Do(t, app, "DELETE", "/workspace/"+f.workspaceID.Hex(), f.admin.Token, nil)
	require.Equal(t, 200, status)

	assert.False(t, services.WorkspaceExists(f.workspaceID), "workspace deletado não existe")
}
