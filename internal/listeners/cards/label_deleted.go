package cards

import (
	"kanban-management/internal/database"
	"kanban-management/internal/events"
	labelEvent "kanban-management/internal/events/label"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleLabelDeleted remove a label de todos os cards do board.
func HandleLabelDeleted(event events.Event) error {
	e := event.(labelEvent.LabelDeletedEvent)

	_, err := database.GetCollection("cards").UpdateMany(e.Ctx, bson.M{
		"board_id":  e.BoardID,
		"label_ids": e.LabelID,
	}, bson.M{
		"$pull": bson.M{"label_ids": e.LabelID},
	})

	return err
}
