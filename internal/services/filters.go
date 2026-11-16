package services

import (
	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// NotDeleted retorna as condições de soft delete para usar em "$or":
//
//	bson.M{"_id": id, "$or": services.NotDeleted()}
//
// Delega para database.NotDeleted (aceita ausente, nil e data zero).
func NotDeleted() []bson.M {
	return database.NotDeleted()
}
