package search

import (
	"context"

	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const backfillBatchSize = 500

// BackfillResult diz quantos documentos receberam (ou receberiam) o search_text.
type BackfillResult struct {
	Cards    int64
	Boards   int64
	Projects int64
}

func str(doc bson.M, key string) string {
	value, _ := doc[key].(string)

	return value
}

// Backfill preenche search_text nos cards, boards e projetos que ainda não têm
// o campo. É idempotente (só toca em quem não tem search_text). Com apply=false
// só conta, sem gravar nada.
func Backfill(ctx context.Context, apply bool) (BackfillResult, error) {
	var (
		result BackfillResult
		err    error
	)

	result.Cards, err = backfillCollection(ctx, "cards", apply, func(doc bson.M) string {
		return BuildText(str(doc, "title"), str(doc, "description"))
	})

	if err != nil {
		return result, err
	}

	result.Boards, err = backfillCollection(ctx, "boards", apply, func(doc bson.M) string {
		return BuildText(str(doc, "name"))
	})

	if err != nil {
		return result, err
	}

	result.Projects, err = backfillCollection(ctx, "projects", apply, func(doc bson.M) string {
		return BuildText(str(doc, "name"), str(doc, "description"))
	})

	return result, err
}

// backfillCollection percorre em lotes (paginando por _id, então os documentos
// atualizados não atrapalham a leitura) e devolve quantos documentos entraram.
func backfillCollection(ctx context.Context, name string, apply bool, text func(bson.M) string) (int64, error) {
	collection := database.GetCollection(name)
	missing := bson.M{"search_text": bson.M{"$exists": false}}

	var (
		count int64
		last  *bson.ObjectID
	)

	for {
		filter := bson.M{"search_text": missing["search_text"]}

		if last != nil {
			filter["_id"] = bson.M{"$gt": *last}
		}

		cursor, err := collection.Find(ctx, filter, options.Find().
			SetSort(bson.D{{Key: "_id", Value: 1}}).
			SetLimit(backfillBatchSize).
			SetProjection(bson.M{"title": 1, "name": 1, "description": 1}))

		if err != nil {
			return count, err
		}

		var docs []bson.M

		if err := cursor.All(ctx, &docs); err != nil {
			return count, err
		}

		if len(docs) == 0 {
			return count, nil
		}

		writes := make([]mongo.WriteModel, 0, len(docs))

		for _, doc := range docs {
			id, ok := doc["_id"].(bson.ObjectID)

			if !ok {
				continue
			}

			last = &id
			count++

			writes = append(writes, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": id, "search_text": bson.M{"$exists": false}}).
				SetUpdate(bson.M{"$set": bson.M{"search_text": text(doc)}}))
		}

		if apply && len(writes) > 0 {
			if _, err := collection.BulkWrite(ctx, writes); err != nil {
				return count, err
			}
		}

		if len(docs) < backfillBatchSize {
			return count, nil
		}
	}
}
