package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegister_NewUser_ReturnsOK(t *testing.T) {
	app := testutil.Setup(t)

	status, env := testutil.Do(t, app, "POST", "/auth/register", "", map[string]any{
		"name": "Novo", "email": "novo-" + testutil.RandomString(4) + "@test.local", "password": "abc12345",
	})

	assert.Equal(t, 200, status)
	assert.True(t, env.Success)
}

func TestRegister_DuplicateEmail_ReturnsConflict(t *testing.T) {
	app := testutil.Setup(t)
	user := testutil.CreateUser(t, app)

	status, env := testutil.Do(t, app, "POST", "/auth/register", "", map[string]any{
		"name": "Outro", "email": user.Email, "password": "abc12345",
	})

	assert.Equal(t, 409, status)
	assert.False(t, env.Success)
}

func TestRegister_InvalidBody_ReturnsBadRequest(t *testing.T) {
	app := testutil.Setup(t)

	status, _ := testutil.Do(t, app, "POST", "/auth/register", "", "isso-nao-e-um-objeto")

	assert.Equal(t, 400, status)
}

func TestLogin(t *testing.T) {
	app := testutil.Setup(t)

	email := "login-" + testutil.RandomString(4) + "@test.local"
	status, _ := testutil.Do(t, app, "POST", "/auth/register", "", map[string]any{
		"name": "Login", "email": email, "password": "senha-correta",
	})
	require.Equal(t, 200, status)

	cases := []struct {
		name       string
		email      string
		password   string
		wantStatus int
	}{
		{"credenciais corretas", email, "senha-correta", 200},
		{"senha incorreta", email, "senha-errada", 401},
		{"e-mail inexistente", "nao-existe-" + testutil.RandomString(3) + "@test.local", "senha-correta", 401},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, env := testutil.Do(t, app, "POST", "/auth/login", "", map[string]any{
				"email": tc.email, "password": tc.password,
			})

			assert.Equal(t, tc.wantStatus, status)

			if tc.wantStatus == 200 {
				assert.NotEmpty(t, env.Map()["token"])
			} else {
				assert.Nil(t, env.Map()["token"])
			}
		})
	}
}

func TestLogin_DeletedUser_ReturnsUnauthorized(t *testing.T) {
	app := testutil.Setup(t)
	user := testutil.CreateUser(t, app)

	status, _ := testutil.Do(t, app, "DELETE", "/users/"+user.ID, user.Token, nil)
	require.Equal(t, 200, status)

	status, _ = testutil.Do(t, app, "POST", "/auth/login", "", map[string]any{
		"email": user.Email, "password": "qualquer",
	})
	assert.Equal(t, 401, status)

	// O token antigo também deixa de valer.
	status, _ = testutil.Do(t, app, "GET", "/auth/me", user.Token, nil)
	assert.Equal(t, 401, status)
}

func TestMe(t *testing.T) {
	app := testutil.Setup(t)
	user := testutil.CreateUser(t, app)

	t.Run("com token válido", func(t *testing.T) {
		status, env := testutil.Do(t, app, "GET", "/auth/me", user.Token, nil)

		assert.Equal(t, 200, status)
		assert.Equal(t, user.Email, env.Map()["email"])
		assert.Nil(t, env.Map()["password"], "a resposta não pode conter a senha")
	})

	t.Run("sem token", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "GET", "/auth/me", "", nil)
		assert.Equal(t, 401, status)
	})

	t.Run("token inválido", func(t *testing.T) {
		status, _ := testutil.Do(t, app, "GET", "/auth/me", "abc.def.ghi", nil)
		assert.Equal(t, 401, status)
	})
}
