package integration_test

import (
	"context"
	"testing"

	"kanban-management/internal/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestDeleteCard_SoftDeletesItsAttachments(t *testing.T) {
	s := newAttachmentScenario(t)
	s.uploadOK(t, s.admin.Token, "a.png", pngBytes)
	s.do(t, "POST", s.path()+"/link", s.admin.Token, map[string]any{"name": "doc", "url": "https://exemplo.com"})

	status, _ := s.do(t, "DELETE", "/card/"+s.cardID, s.admin.Token, nil)
	require.Equal(t, 200, status)

	status, _ = s.do(t, "GET", s.path(), s.admin.Token, nil)
	assert.Equal(t, 404, status, "card deletado não lista anexos")

	cardOID, _ := bson.ObjectIDFromHex(s.cardID)
	total, err := database.GetCollection("attachments").CountDocuments(context.Background(), bson.M{"card_id": cardOID})
	require.NoError(t, err)
	deleted, err := database.GetCollection("attachments").CountDocuments(context.Background(), bson.M{
		"card_id": cardOID, "deleted_at": bson.M{"$exists": true, "$ne": nil},
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), total)
	assert.Equal(t, total, deleted, "todos os anexos devem ter deleted_at")
}
