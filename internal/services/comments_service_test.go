package services_test

import (
	"strings"
	"testing"

	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestValidateCommentContent(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"normal", "Olá", "Olá", false},
		{"aparado", "  Olá  ", "Olá", false},
		{"vazio", "", "", true},
		{"só espaços", "   \n\t ", "", true},
		{"5000 caracteres", strings.Repeat("a", 5000), strings.Repeat("a", 5000), false},
		{"5001 caracteres", strings.Repeat("a", 5001), "", true},
		{"5000 runes multibyte", strings.Repeat("é", 5000), strings.Repeat("é", 5000), false},
		{"o trim vem antes do limite", " " + strings.Repeat("a", 5000) + " ", strings.Repeat("a", 5000), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := services.ValidateCommentContent(tc.in)

			if tc.wantErr {
				assert.ErrorIs(t, err, services.ErrInvalidComment)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestToCommentResponses_RemovedAuthor(t *testing.T) {
	alive, removed := bson.NewObjectID(), bson.NewObjectID()
	authors := map[bson.ObjectID]models.CommentAuthor{alive: {ID: alive, Name: "Ana"}}

	responses := services.ToCommentResponses([]models.Comment{
		{AuthorID: &alive}, {AuthorID: &removed}, {},
	}, authors)

	assert.Len(t, responses, 3)
	assert.Equal(t, "Ana", responses[0].Author.Name)
	assert.Equal(t, models.RemovedUserName, responses[1].Author.Name)
	assert.Equal(t, removed, responses[1].Author.ID)
	assert.Nil(t, responses[2].Author)
	assert.NotNil(t, services.ToCommentResponses(nil, authors), "lista vazia nunca é nil")
}
