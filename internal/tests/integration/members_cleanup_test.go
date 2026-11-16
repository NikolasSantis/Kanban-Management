package integration_test

import (
	"context"
	"testing"

	"kanban-management/internal/database"
	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// membersState devolve quantos membros o escopo tem no total e quantos têm deleted_at.
func membersState(t *testing.T, collection, field, id string) (total, deleted int64) {
	t.Helper()

	oid, err := bson.ObjectIDFromHex(id)
	require.NoError(t, err)

	ctx := context.Background()
	coll := database.GetCollection(collection)

	total, err = coll.CountDocuments(ctx, bson.M{field: oid})
	require.NoError(t, err)

	deleted, err = coll.CountDocuments(ctx, bson.M{field: oid, "deleted_at": bson.M{"$exists": true, "$ne": nil}})
	require.NoError(t, err)

	return total, deleted
}

func TestDeleteProject_SoftDeletesItsMembers(t *testing.T) {
	s := newBoardScenario(t) // admin, manager e member no projeto

	total, deleted := membersState(t, "project_members", "project_id", s.projectID)
	require.Equal(t, int64(3), total)
	require.Equal(t, int64(0), deleted)

	status, _ := s.do(t, "DELETE", "/project/"+s.projectID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	total, deleted = membersState(t, "project_members", "project_id", s.projectID)
	assert.Equal(t, int64(3), total, "os registros continuam no banco")
	assert.Equal(t, total, deleted, "todos os membros devem ter deleted_at")

	_, env := s.do(t, "GET", "/project/", s.member.Token, nil)
	assert.False(t, containsID(env.List(), s.projectID))
}

func TestDeleteProject_KeepsEarlierMemberRemovalDate(t *testing.T) {
	s := newBoardScenario(t)

	status, _ := s.do(t, "DELETE", "/project/member/"+s.projectID, s.admin.Token, map[string]any{"user_id": s.member.ID})
	require.Equal(t, 200, status)

	oid, _ := bson.ObjectIDFromHex(s.projectID)
	memberOID, _ := bson.ObjectIDFromHex(s.member.ID)

	var before bson.M
	require.NoError(t, database.GetCollection("project_members").FindOne(context.Background(),
		bson.M{"project_id": oid, "user_id": memberOID}).Decode(&before))

	status, _ = s.do(t, "DELETE", "/project/"+s.projectID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	var after bson.M
	require.NoError(t, database.GetCollection("project_members").FindOne(context.Background(),
		bson.M{"project_id": oid, "user_id": memberOID}).Decode(&after))

	assert.Equal(t, before["deleted_at"], after["deleted_at"], "quem já tinha saído mantém a data de saída")
}

func TestDeleteWorkspace_SoftDeletesItsMembers(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	member := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)

	status, _ := testutil.Do(t, app, "POST", "/workspace/member/"+workspaceID, owner.Token, map[string]any{"user_id": member.ID, "role": "member"})
	require.Equal(t, 200, status)

	total, deleted := membersState(t, "workspaces_members", "workspace_id", workspaceID)
	require.Equal(t, int64(2), total)
	require.Equal(t, int64(0), deleted)

	status, _ = testutil.Do(t, app, "DELETE", "/workspace/"+workspaceID, owner.Token, nil)
	require.Equal(t, 200, status)

	total, deleted = membersState(t, "workspaces_members", "workspace_id", workspaceID)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, total, deleted, "todos os membros devem ter deleted_at")

	_, env := testutil.Do(t, app, "GET", "/workspace/member", member.Token, nil)
	assert.False(t, containsID(env.List(), workspaceID), "o workspace deletado não aparece para o membro")
}
