package workspace

import (
	"kanban-management/internal/database"
	"kanban-management/internal/events"
	"kanban-management/internal/events/workspace"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func HandleWorkspaceDeleted(event events.Event) error {
	e := event.(workspace.WorkspaceDeletedEvent)

	collection := database.GetCollection("workspaces_members")

	now := time.Now()

	// Só os membros ativos: não sobrescreve o deleted_at de quem já tinha saído.
	_, err := collection.UpdateMany(e.Ctx, bson.M{
		"workspace_id": e.WorkspaceID,
		"$or": []bson.M{ // mesmo filtro de services.NotDeleted (sem importar services: ciclo)
			{"deleted_at": bson.M{"$exists": false}},
			{"deleted_at": nil},
		},
	}, bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"updated_at": now,
		},
	})

	if err != nil {
		return err
	}

	return nil
}
