package services

import (
	"context"

	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// UserExists informa se o usuário existe e não foi deletado (soft delete).
func UserExists(ctx context.Context, userID bson.ObjectID) bool {
	count, err := database.GetCollection("users").CountDocuments(ctx, bson.M{
		"_id": userID,
		"$or": NotDeleted(),
	})

	return err == nil && count > 0
}
