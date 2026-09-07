package services

import (
	"context"
	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func WorkspaceExists(workspaceID bson.ObjectID) bool {
	collection := database.GetCollection("workspaces")
	ctx := context.Background()

	result := collection.FindOne(ctx, bson.M{
		"workspace_id": workspaceID,
	})

	if result.Err() == mongo.ErrNoDocuments {
		return false
	}

	if result.Err() != nil {
		return false
	}

	return true
}
