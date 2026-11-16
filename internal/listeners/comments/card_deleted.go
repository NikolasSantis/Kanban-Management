package comments

import (
	"time"

	"kanban-management/internal/database"
	"kanban-management/internal/events"
	cardEvent "kanban-management/internal/events/card"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleCardDeleted faz o soft delete dos comentários ativos do card.
func HandleCardDeleted(event events.Event) error {
	e := event.(cardEvent.CardDeletedEvent)

	now := time.Now()

	_, err := database.GetCollection("comments").UpdateMany(e.Ctx, bson.M{
		"card_id": e.CardID,
		"$or":     database.NotDeleted(),
	}, bson.M{
		"$set": bson.M{"deleted_at": now, "updated_at": now},
	})

	return err
}
