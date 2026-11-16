package integration_test

import (
	"context"
	"testing"
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/search"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBackfillSearchText(t *testing.T) {
	s := newCardScenario(t)
	ctx := context.Background()

	projectOID, _ := bson.ObjectIDFromHex(s.projectID)
	boardOID, _ := bson.ObjectIDFromHex(s.boardID)
	workspaceOID, _ := bson.ObjectIDFromHex(s.workspaceID)

	// Documentos "antigos", criados direto no banco de teste sem search_text.
	legacyCard, err := database.GetCollection("cards").InsertOne(ctx, bson.M{
		"project_id": projectOID, "board_id": boardOID, "column_id": "todo", "position": 9000.0,
		"title": "Cartão Antigo", "description": "Migração pendente", "assignee_ids": bson.A{}, "label_ids": bson.A{},
		"priority": "medium", "priority_level": 2, "created_at": time.Now(), "updated_at": time.Now(),
	})
	require.NoError(t, err)

	legacyBoard, err := database.GetCollection("boards").InsertOne(ctx, bson.M{
		"project_id": projectOID, "name": "Quadro Legado", "columns": bson.A{}, "created_at": time.Now(), "updated_at": time.Now(),
	})
	require.NoError(t, err)

	legacyProject, err := database.GetCollection("projects").InsertOne(ctx, bson.M{
		"workspace_id": workspaceOID, "owner_id": projectOID, "name": "Projeto Legado", "description": "Descrição Antiga",
		"created_at": time.Now(), "updated_at": time.Now(),
	})
	require.NoError(t, err)

	searchText := func(collection string, id any) (string, bool) {
		var doc bson.M
		require.NoError(t, database.GetCollection(collection).FindOne(ctx, bson.M{"_id": id}).Decode(&doc))
		value, ok := doc["search_text"].(string)

		return value, ok
	}

	t.Run("dry-run só conta e não grava", func(t *testing.T) {
		result, err := search.Backfill(ctx, false)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, result.Cards, int64(1))
		assert.GreaterOrEqual(t, result.Boards, int64(1))
		assert.GreaterOrEqual(t, result.Projects, int64(1))

		_, has := searchText("cards", legacyCard.InsertedID)
		assert.False(t, has, "o dry-run não pode gravar")
	})

	t.Run("apply preenche com o texto normalizado", func(t *testing.T) {
		_, err := search.Backfill(ctx, true)
		require.NoError(t, err)

		text, _ := searchText("cards", legacyCard.InsertedID)
		assert.Equal(t, "cartao antigo migracao pendente", text)

		text, _ = searchText("boards", legacyBoard.InsertedID)
		assert.Equal(t, "quadro legado", text)

		text, _ = searchText("projects", legacyProject.InsertedID)
		assert.Equal(t, "projeto legado descricao antiga", text)
	})

	t.Run("é idempotente: a segunda execução não altera nada", func(t *testing.T) {
		result, err := search.Backfill(ctx, true)

		require.NoError(t, err)
		assert.Equal(t, search.BackfillResult{}, result)
	})

	t.Run("depois do backfill o card antigo é encontrado pela busca", func(t *testing.T) {
		_, env := s.searchCards(t, s.admin.Token, "migracao")

		assert.Equal(t, []string{"Cartão Antigo"}, titlesOf(env))
	})
}
