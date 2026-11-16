package boards

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	boardEvent "kanban-management/internal/events/board"
	projectEvent "kanban-management/internal/events/project"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CascadeProjectDeleted faz o soft delete dos boards ativos do projeto e
// dispara board.deleted para cada um (que apaga cards e labels).
// Recebe o dispatcher porque os listeners não podem importar services (ciclo).
func CascadeProjectDeleted(dispatcher *events.Dispatcher) events.Handler {
	return func(event events.Event) error {
		e := event.(projectEvent.ProjectDeletedEvent)

		collection := database.GetCollection("boards")

		cursor, err := collection.Find(e.Ctx, bson.M{
			"project_id": e.ProjectID,
			"$or":        database.NotDeleted(),
		})

		if err != nil {
			return err
		}

		defer cursor.Close(e.Ctx)

		var boards []models.Board

		if err := cursor.All(e.Ctx, &boards); err != nil {
			return err
		}

		for _, board := range boards {
			now := time.Now()

			_, err := collection.UpdateByID(e.Ctx, *board.ID, bson.M{
				"$set": bson.M{"deleted_at": now, "updated_at": now},
			})

			if err != nil {
				return err
			}

			if err := dispatcher.Dispatch(boardEvent.BoardDeletedEvent{BoardID: *board.ID, Ctx: e.Ctx}); err != nil {
				return err
			}
		}

		return nil
	}
}
