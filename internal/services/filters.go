package services

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// NotDeleted retorna as condições de soft delete para usar em "$or":
//
//	bson.M{"_id": id, "$or": services.NotDeleted()}
//
// Aceita deleted_at ausente, nil ou data zero (alguns documentos antigos foram
// gravados com 0001-01-01 por causa de models sem ponteiro/tag incorreta).
func NotDeleted() []bson.M {
	return []bson.M{
		{"deleted_at": bson.M{"$exists": false}},
		{"deleted_at": nil},
		{"deleted_at": time.Time{}},
	}
}
