package project

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	projectEvent "kanban-management/internal/events/project"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleProjectDeleted faz o soft delete dos membros ativos do projeto.
func HandleProjectDeleted(event events.Event) error {
	e := event.(projectEvent.ProjectDeletedEvent)

	now := time.Now()

	_, err := database.GetCollection("project_members").UpdateMany(e.Ctx, bson.M{
		"project_id": e.ProjectID,
		"$or":        database.NotDeleted(),
	}, bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"updated_at": now,
		},
	})

	return err
}
