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

func TestMigrateCardPriorities(t *testing.T) {
	s := newCardScenario(t)
	withPriority := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "ja-tem")
	status, _ := s.do(t, "PATCH", "/card/"+withPriority, s.admin.Token, map[string]any{"priority": "urgent"})
	require.Equal(t, 200, status)

	// Cria um card "antigo" direto no banco de teste, sem priority nem priority_level.
	boardOID, _ := bson.ObjectIDFromHex(s.boardID)
	projectOID, _ := bson.ObjectIDFromHex(s.projectID)
	legacy, err := database.GetCollection("cards").InsertOne(context.Background(), bson.M{
		"board_id": boardOID, "project_id": projectOID, "column_id": "todo", "title": "antigo",
		"position": 9000.0, "assignee_ids": bson.A{},
	})
	require.NoError(t, err)

	ctx := context.Background()

	count, err := database.CountCardsWithoutPriority(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(1), "a contagem enxerga o card antigo sem alterar nada")

	changed, err := database.MigrateCardPriorities(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, changed, int64(1))

	var doc bson.M
	require.NoError(t, database.GetCollection("cards").FindOne(ctx, bson.M{"_id": legacy.InsertedID}).Decode(&doc))
	assert.Equal(t, "medium", doc["priority"])
	assert.Equal(t, int32(2), doc["priority_level"])

	assert.Equal(t, "urgent", s.cardJSON(t, withPriority)["priority"], "quem já tinha prioridade não é tocado")

	changed, err = database.MigrateCardPriorities(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), changed, "idempotente: a segunda execução não altera nada")
}
