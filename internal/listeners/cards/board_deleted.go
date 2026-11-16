package cards

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	boardEvent "kanban-management/internal/events/board"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleBoardDeleted faz o soft delete dos cards ativos do board.
func HandleBoardDeleted(event events.Event) error {
	e := event.(boardEvent.BoardDeletedEvent)

	now := time.Now()

	_, err := database.GetCollection("cards").UpdateMany(e.Ctx, bson.M{
		"board_id": e.BoardID,
		"$or": []bson.M{ // não importa services (ciclo): mesmo filtro de services.NotDeleted
			{"deleted_at": bson.M{"$exists": false}},
			{"deleted_at": nil},
		},
	}, bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"updated_at": now,
		},
	})

	return err
}
