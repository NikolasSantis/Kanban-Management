package project

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type ProjectCreatedEvent struct {
	ProjectID bson.ObjectID
	UserID    bson.ObjectID
	Ctx       context.Context
}

func (e ProjectCreatedEvent) Name() string {
	return "project.created"
}
