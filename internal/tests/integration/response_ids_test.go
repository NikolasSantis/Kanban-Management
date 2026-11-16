package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const zeroObjectID = "000000000000000000000000"

func TestCreateWorkspace_ResponseHasRealID(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	name := "ws-id-" + testutil.RandomString(3)

	status, env := testutil.Do(t, app, "POST", "/workspace/", owner.Token, map[string]any{"name": name})
	require.Equal(t, 200, status)

	id, _ := env.Map()["id"].(string)
	assert.NotEmpty(t, id)
	assert.NotEqual(t, zeroObjectID, id, "a resposta não pode trazer o id zerado")

	_, list := testutil.Do(t, app, "GET", "/workspace/", owner.Token, nil)
	assert.True(t, containsID(list.List(), id), "o id da resposta deve ser o do workspace criado")
}

func TestCreateProject_ResponseHasRealID(t *testing.T) {
	app := testutil.Setup(t)
	owner := testutil.CreateUser(t, app)
	workspaceID := testutil.CreateWorkspace(t, app, owner.Token)

	status, env := testutil.Do(t, app, "POST", "/project/", owner.Token, map[string]any{
		"workspace_id": workspaceID, "name": "proj-id-" + testutil.RandomString(3),
	})
	require.Equal(t, 201, status)

	id, _ := env.Map()["id"].(string)
	assert.NotEmpty(t, id)
	assert.NotEqual(t, zeroObjectID, id, "a resposta não pode trazer o id zerado")

	_, list := testutil.Do(t, app, "GET", "/project/", owner.Token, nil)
	assert.True(t, containsID(list.List(), id), "o id da resposta deve ser o do projeto criado")
}
