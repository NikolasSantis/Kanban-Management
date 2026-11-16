package board

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type BoardDeletedEvent struct {
	BoardID bson.ObjectID
	Ctx     context.Context
}

func (e BoardDeletedEvent) Name() string {
	return "board.deleted"
}
