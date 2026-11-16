package filters

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var roles = []string{"member", "manager", "admin"}

func TestParseRoles(t *testing.T) {
	got, err := ParseRoles(" admin , manager,admin", roles)
	assert.NoError(t, err)
	assert.Equal(t, []string{"admin", "manager"}, got)

	got, err = ParseRoles("", roles)
	assert.NoError(t, err)
	assert.Nil(t, got)

	_, err = ParseRoles("admin,dono", roles)
	require.Error(t, err)
	queryErr, ok := AsQueryError(err)
	require.True(t, ok)
	assert.Equal(t, "Invalid role. Allowed: member, manager, admin", queryErr.Message)

	_, err = ParseRoles(strings.Repeat("member,", 1)+strings.Repeat("x,", 25), roles)
	assert.Error(t, err)
}

func TestParseStatuses(t *testing.T) {
	got, err := ParseStatuses("active, archived")
	assert.NoError(t, err)
	assert.Equal(t, []string{"active", "archived"}, got)

	got, err = ParseStatuses("  ")
	assert.NoError(t, err)
	assert.Nil(t, got)

	_, err = ParseStatuses(strings.Repeat("a", 51))
	assert.Error(t, err)
}

func TestParseObjectID(t *testing.T) {
	id, err := ParseObjectID(u1.Hex(), "workspace_id")
	assert.NoError(t, err)
	assert.Equal(t, u1, *id)

	id, err = ParseObjectID("", "workspace_id")
	assert.NoError(t, err)
	assert.Nil(t, id)

	_, err = ParseObjectID("xyz", "workspace_id")
	require.Error(t, err)
	queryErr, _ := AsQueryError(err)
	assert.Equal(t, "Invalid workspace_id", queryErr.Message)
}

func TestParseAuthorID(t *testing.T) {
	id, err := ParseAuthorID("me", me)
	assert.NoError(t, err)
	assert.Equal(t, me, *id)

	id, err = ParseAuthorID(u1.Hex(), me)
	assert.NoError(t, err)
	assert.Equal(t, u1, *id)

	id, err = ParseAuthorID("", me)
	assert.NoError(t, err)
	assert.Nil(t, id)

	_, err = ParseAuthorID("nobody", me)
	assert.Error(t, err)
}

func TestParseKind(t *testing.T) {
	for _, ok := range []string{"", "file", "link", " LINK "} {
		_, err := ParseKind(ok)
		assert.NoError(t, err, ok)
	}

	kind, _ := ParseKind(" LINK ")
	assert.Equal(t, "link", kind)

	_, err := ParseKind("pdf")
	assert.Error(t, err)
}
