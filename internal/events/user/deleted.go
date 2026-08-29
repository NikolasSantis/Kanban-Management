package user

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type UserDeletedEvent struct {
	UserID bson.ObjectID
	Ctx    context.Context
}

func (e UserDeletedEvent) Name() string {
	return "user.deleted"
}
