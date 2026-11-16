package projects

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	projectEvent "kanban-management/internal/events/project"
	workspaceEvent "kanban-management/internal/events/workspace"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CascadeWorkspaceDeleted faz o soft delete dos projetos ativos do workspace e
// dispara project.deleted para cada um (que apaga membros, boards, cards e labels).
// Recebe o dispatcher porque os listeners não podem importar services (ciclo).
func CascadeWorkspaceDeleted(dispatcher *events.Dispatcher) events.Handler {
	return func(event events.Event) error {
		e := event.(workspaceEvent.WorkspaceDeletedEvent)

		collection := database.GetCollection("projects")

		cursor, err := collection.Find(e.Ctx, bson.M{
			"workspace_id": e.WorkspaceID,
			"$or":          database.NotDeleted(),
		})

		if err != nil {
			return err
		}

		defer cursor.Close(e.Ctx)

		var projects []models.Project

		if err := cursor.All(e.Ctx, &projects); err != nil {
			return err
		}

		for _, project := range projects {
			now := time.Now()

			_, err := collection.UpdateByID(e.Ctx, project.ID, bson.M{
				"$set": bson.M{"deleted_at": now, "updated_at": now},
			})

			if err != nil {
				return err
			}

			if err := dispatcher.Dispatch(projectEvent.ProjectDeletedEvent{ProjectID: project.ID, Ctx: e.Ctx}); err != nil {
				return err
			}
		}

		return nil
	}
}
