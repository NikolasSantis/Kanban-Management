package label

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type LabelDeletedEvent struct {
	LabelID bson.ObjectID
	BoardID bson.ObjectID
	Ctx     context.Context
}

func (e LabelDeletedEvent) Name() string {
	return "label.deleted"
}
