package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// commentScenario: cenário de cards com um card já criado.
type commentScenario struct {
	cardScenario
	cardID string
}

func newCommentScenario(t *testing.T) commentScenario {
	t.Helper()

	s := newCardScenario(t)

	return commentScenario{
		cardScenario: s,
		cardID:       testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "card"),
	}
}

func (s commentScenario) commentsPath() string { return "/comment/card/" + s.cardID }

func (s commentScenario) comment(t *testing.T, token, content string) string {
	t.Helper()

	status, env := s.do(t, "POST", s.commentsPath(), token, map[string]any{"content": content})
	require.Equal(t, 201, status, "create comment: %v", env.Error)

	return env.Map()["id"].(string)
}

func (s commentScenario) contents(t *testing.T, token string) []string {
	t.Helper()

	status, env := s.do(t, "GET", s.commentsPath(), token, nil)
	require.Equal(t, 200, status)

	var out []string
	for _, c := range env.List() {
		out = append(out, c["content"].(string))
	}

	return out
}

func TestCreateComment(t *testing.T) {
	s := newCommentScenario(t)

	t.Run("member comenta; o texto é aparado e a resposta traz o autor", func(t *testing.T) {
		status, env := s.do(t, "POST", s.commentsPath(), s.member.Token, map[string]any{"content": "  Olá, time  "})

		require.Equal(t, 201, status)
		comment := env.Map()
		assert.Equal(t, "Olá, time", comment["content"])
		assert.Equal(t, s.member.ID, comment["author_id"])
		assert.Equal(t, s.cardID, comment["card_id"])
		assert.Equal(t, s.projectID, comment["project_id"])
		assert.Equal(t, s.boardID, comment["board_id"])
		assert.Equal(t, s.member.ID, comment["author"].(map[string]any)["id"])
		assert.NotEmpty(t, comment["author"].(map[string]any)["name"])
		assert.NotContains(t, comment, "edited_at")
		assert.NotContains(t, comment, "deleted_by")
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"só espaços", s.commentsPath(), s.admin.Token, map[string]any{"content": "   "}, 400},
		{"vazio", s.commentsPath(), s.admin.Token, map[string]any{"content": ""}, 400},
		{"sem content", s.commentsPath(), s.admin.Token, map[string]any{}, 400},
		{"5001 caracteres", s.commentsPath(), s.admin.Token, map[string]any{"content": strings.Repeat("a", 5001)}, 400},
		{"5000 caracteres", s.commentsPath(), s.admin.Token, map[string]any{"content": strings.Repeat("a", 5000)}, 201},
		{"corpo inválido", s.commentsPath(), s.admin.Token, "texto", 400},
		{"não-membro recebe 404", s.commentsPath(), s.outsider.Token, map[string]any{"content": "oi"}, 404},
		{"sem autenticação", s.commentsPath(), "", map[string]any{"content": "oi"}, 401},
		{"id de card inválido", "/comment/card/xyz", s.admin.Token, map[string]any{"content": "oi"}, 400},
		{"card inexistente", "/comment/card/000000000000000000000001", s.admin.Token, map[string]any{"content": "oi"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "POST", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestGetComments(t *testing.T) {
	s := newCommentScenario(t)
	s.comment(t, s.admin.Token, "primeiro")
	s.comment(t, s.member.Token, "segundo")
	s.comment(t, s.manager.Token, "terceiro")
	s.comment(t, s.member.Token, "quarto")

	t.Run("mais antigos primeiro, com o autor de cada um", func(t *testing.T) {
		status, env := s.do(t, "GET", s.commentsPath(), s.member.Token, nil)

		require.Equal(t, 200, status)
		list := env.List()
		require.Len(t, list, 4)

		assert.Equal(t, "primeiro", list[0]["content"])
		assert.Equal(t, "quarto", list[3]["content"])

		authors := map[string]string{s.admin.ID: "", s.member.ID: "", s.manager.ID: ""}
		for _, comment := range list {
			author := comment["author"].(map[string]any)
			assert.Equal(t, comment["author_id"], author["id"])
			assert.NotEmpty(t, author["name"])
			assert.Len(t, author, 2, "o autor só tem id e name")
			authors[author["id"].(string)] = author["name"].(string)
		}
		assert.Len(t, authors, 3)
	})

	t.Run("a resposta não traz e-mail nem senha", func(t *testing.T) {
		status, env := s.do(t, "GET", s.commentsPath(), s.member.Token, nil)
		require.Equal(t, 200, status)

		raw := strings.ToLower(string(env.Data))
		assert.NotContains(t, raw, "@test.local", "nenhum e-mail pode vazar")
		assert.NotContains(t, raw, "password")
		assert.NotContains(t, raw, "email")
	})

	t.Run("não-membro recebe 404", func(t *testing.T) {
		status, _ := s.do(t, "GET", s.commentsPath(), s.outsider.Token, nil)
		assert.Equal(t, 404, status)
	})

	t.Run("card sem comentários devolve lista vazia", func(t *testing.T) {
		other := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "vazio")

		status, env := s.do(t, "GET", "/comment/card/"+other, s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.JSONEq(t, "[]", string(env.Data))
	})
}

func TestGetComments_RemovedAuthorStillAppears(t *testing.T) {
	s := newCommentScenario(t)

	leaver := testutil.CreateUser(t, s.app)
	testutil.AddProjectMember(t, s.app, s.admin.Token, s.projectID, leaver.ID, "member")
	s.comment(t, leaver.Token, "deixei isto aqui")

	status, _ := s.do(t, "DELETE", "/users/"+leaver.ID, leaver.Token, nil)
	require.Equal(t, 200, status)

	status, env := s.do(t, "GET", s.commentsPath(), s.admin.Token, nil)

	require.Equal(t, 200, status)
	require.Len(t, env.List(), 1)
	author := env.List()[0]["author"].(map[string]any)
	assert.Equal(t, "Usuário removido", author["name"])
	assert.Equal(t, leaver.ID, author["id"])
}

func TestGetComments_ReturnsOnlyTheLast100(t *testing.T) {
	s := newCommentScenario(t)

	cardOID, _ := bson.ObjectIDFromHex(s.cardID)
	authorOID, _ := bson.ObjectIDFromHex(s.admin.ID)
	base := time.Now().Add(-time.Hour)

	docs := make([]any, 0, 105)
	for i := 0; i < 105; i++ {
		at := base.Add(time.Duration(i) * time.Second)
		docs = append(docs, bson.M{
			"card_id": cardOID, "author_id": authorOID, "content": fmt.Sprintf("c%03d", i),
			"created_at": at, "updated_at": at,
		})
	}

	_, err := database.GetCollection("comments").InsertMany(context.Background(), docs)
	require.NoError(t, err)

	contents := s.contents(t, s.admin.Token)

	require.Len(t, contents, 100)
	assert.Equal(t, "c005", contents[0], "os 5 mais antigos ficam de fora")
	assert.Equal(t, "c104", contents[99])
}

func TestPatchComment(t *testing.T) {
	s := newCommentScenario(t)
	commentID := s.comment(t, s.member.Token, "original")
	path := "/comment/" + commentID

	t.Run("o autor edita; edited_at é preenchido e created_at não muda", func(t *testing.T) {
		_, before := s.do(t, "GET", s.commentsPath(), s.member.Token, nil)
		createdAt := before.List()[0]["created_at"]
		require.Nil(t, before.List()[0]["edited_at"])

		status, env := s.do(t, "PATCH", path, s.member.Token, map[string]any{"content": "  editado  "})

		require.Equal(t, 200, status)
		assert.Equal(t, "editado", env.Map()["content"])
		assert.NotNil(t, env.Map()["edited_at"])
		assert.Equal(t, createdAt, env.Map()["created_at"])
		assert.NotEmpty(t, env.Map()["author"].(map[string]any)["name"])
	})

	t.Run("outra pessoa não edita, nem admin", func(t *testing.T) {
		for _, token := range []string{s.admin.Token, s.manager.Token} {
			status, _ := s.do(t, "PATCH", path, token, map[string]any{"content": "invadido"})
			assert.Equal(t, 403, status)
		}

		assert.Equal(t, []string{"editado"}, s.contents(t, s.admin.Token))
	})

	cases := []struct {
		name       string
		path       string
		token      string
		body       any
		wantStatus int
	}{
		{"só espaços", path, s.member.Token, map[string]any{"content": "  "}, 400},
		{"sem content", path, s.member.Token, map[string]any{}, 400},
		{"JSON inválido", path, s.member.Token, "texto", 400},
		{"não-membro recebe 404", path, s.outsider.Token, map[string]any{"content": "x"}, 404},
		{"id inválido", "/comment/xyz", s.member.Token, map[string]any{"content": "x"}, 400},
		{"comentário inexistente", "/comment/000000000000000000000001", s.member.Token, map[string]any{"content": "x"}, 404},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := s.do(t, "PATCH", tc.path, tc.token, tc.body)
			assert.Equal(t, tc.wantStatus, status)
		})
	}
}

func TestDeleteComment(t *testing.T) {
	s := newCommentScenario(t)

	t.Run("member não apaga comentário de outro", func(t *testing.T) {
		id := s.comment(t, s.admin.Token, "do admin")

		status, _ := s.do(t, "DELETE", "/comment/"+id, s.member.Token, nil)

		assert.Equal(t, 403, status)
	})

	t.Run("member apaga o próprio", func(t *testing.T) {
		id := s.comment(t, s.member.Token, "do member")

		status, _ := s.do(t, "DELETE", "/comment/"+id, s.member.Token, nil)

		assert.Equal(t, 200, status)
	})

	t.Run("manager e admin apagam de qualquer um (moderação)", func(t *testing.T) {
		first := s.comment(t, s.member.Token, "m1")
		second := s.comment(t, s.member.Token, "m2")

		status, _ := s.do(t, "DELETE", "/comment/"+first, s.manager.Token, nil)
		assert.Equal(t, 200, status)

		status, _ = s.do(t, "DELETE", "/comment/"+second, s.admin.Token, nil)
		assert.Equal(t, 200, status)
	})

	t.Run("não-membro recebe 404 e id inválido 400", func(t *testing.T) {
		id := s.comment(t, s.admin.Token, "x")

		status, _ := s.do(t, "DELETE", "/comment/"+id, s.outsider.Token, nil)
		assert.Equal(t, 404, status)

		status, _ = s.do(t, "DELETE", "/comment/xyz", s.admin.Token, nil)
		assert.Equal(t, 400, status)
	})

	t.Run("soft delete: some da listagem, continua no banco com deleted_by", func(t *testing.T) {
		id := s.comment(t, s.member.Token, "vai-sumir")

		status, _ := s.do(t, "DELETE", "/comment/"+id, s.manager.Token, nil)
		require.Equal(t, 200, status)

		assert.NotContains(t, s.contents(t, s.admin.Token), "vai-sumir")

		status, _ = s.do(t, "DELETE", "/comment/"+id, s.manager.Token, nil)
		assert.Equal(t, 404, status, "deletar duas vezes devolve 404")

		status, _ = s.do(t, "PATCH", "/comment/"+id, s.member.Token, map[string]any{"content": "x"})
		assert.Equal(t, 404, status, "comentário deletado não pode ser editado")

		oid, _ := bson.ObjectIDFromHex(id)
		var doc bson.M
		require.NoError(t, database.GetCollection("comments").FindOne(context.Background(), bson.M{"_id": oid}).Decode(&doc))
		assert.NotNil(t, doc["deleted_at"])
		assert.Equal(t, s.manager.ID, doc["deleted_by"].(bson.ObjectID).Hex())
	})
}
