package services

import (
	"context"
	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func WorkspaceExists(workspaceID bson.ObjectID) bool {
	collection := database.GetCollection("workspaces")
	ctx := context.Background()

	result := collection.FindOne(ctx, bson.M{
		"_id": workspaceID,
		"$or": NotDeleted(),
	})

	return result.Err() == nil
}
