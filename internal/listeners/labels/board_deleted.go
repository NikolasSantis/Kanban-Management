package labels

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	boardEvent "kanban-management/internal/events/board"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleBoardDeleted faz o soft delete das labels ativas do board.
// (active=false também libera o nome no índice único parcial.)
func HandleBoardDeleted(event events.Event) error {
	e := event.(boardEvent.BoardDeletedEvent)

	now := time.Now()

	_, err := database.GetCollection("labels").UpdateMany(e.Ctx, bson.M{
		"board_id": e.BoardID,
		"$or":      database.NotDeleted(),
	}, bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"updated_at": now,
			"active":     false,
		},
	})

	return err
}
