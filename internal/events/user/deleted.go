package events

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserDeleted struct {
	UserID bson.ObjectID
	Ctx    context.Context
}

func (e UserDeleted) Name() string {
	return "user.deleted"
}
