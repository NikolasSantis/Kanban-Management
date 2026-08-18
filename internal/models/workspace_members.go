package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type WorkspaceMember struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      bson.ObjectID `bson:"user_id" json:"user_id"`
	WorkspaceID bson.ObjectID `bson:"workspace_id" json:"workspace_id"`
	Role        string        `bson:"role" json:"role"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at"`
	DeletedAt   time.Time     `bson:"deleted_at,omitempty" json:"deleted_at"`
}
