package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealth_IsPublicAndChecksTheDatabase(t *testing.T) {
	app := testutil.Setup(t)

	status, env := testutil.Do(t, app, "GET", "/health", "", nil)

	require.Equal(t, 200, status, "sem token")
	assert.True(t, env.Success)
	assert.Equal(t, "ok", env.Map()["status"])
}
