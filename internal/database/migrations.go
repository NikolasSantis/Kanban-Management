package database

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// cardsWithoutPriority seleciona os cards criados antes da etapa 07.
var cardsWithoutPriority = bson.M{"priority": bson.M{"$exists": false}}

// CountCardsWithoutPriority conta (só leitura) quantos cards a migração alteraria.
func CountCardsWithoutPriority(ctx context.Context) (int64, error) {
	return GetCollection("cards").CountDocuments(ctx, cardsWithoutPriority)
}

// MigrateCardPriorities dá prioridade "medium" (nível 2) aos cards que ainda
// não têm o campo. É idempotente: só toca em quem não tem priority.
// Devolve quantos cards foram alterados.
func MigrateCardPriorities(ctx context.Context) (int64, error) {
	result, err := GetCollection("cards").UpdateMany(ctx, cardsWithoutPriority, bson.M{
		"$set": bson.M{"priority": "medium", "priority_level": 2},
	})

	if err != nil {
		return 0, err
	}

	return result.ModifiedCount, nil
}
