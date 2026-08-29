package workspace

import (
	"kanban-management/internal/database"
	"kanban-management/internal/events"
	userEvents "kanban-management/internal/events/user"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func HandleUserDeleted(event events.Event) error {
	e := event.(userEvents.UserDeletedEvent)

	collection := database.GetCollection("workspaces_members")

	_, err := collection.UpdateMany(
		e.Ctx,
		bson.M{
			"user_id": e.UserID,
		},
		bson.M{
			"$set": bson.M{
				"deleted_at": time.Now(),
			},
		},
	)

	if err != nil {
		return err
	}

	return nil
}
