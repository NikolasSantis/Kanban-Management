package project

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ProjectDeletedEvent struct {
	ProjectID bson.ObjectID
	Ctx       context.Context
}

func (e ProjectDeletedEvent) Name() string {
	return "project.deleted"
}
