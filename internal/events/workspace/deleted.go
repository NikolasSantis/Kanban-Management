package workspace

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type WorkspaceDeletedEvent struct {
	WorkspaceID bson.ObjectID
	Ctx         context.Context
}

func (e WorkspaceDeletedEvent) Name() string {
	return "workspace.deleted"
}
