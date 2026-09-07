package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Project struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	WorkspaceID bson.ObjectID `bson:"workspace_id" json:"workspace_id"`
	Owner_id    bson.ObjectID `bson:"owner_id" json:"owner_id"`
	Name        string        `bson:"name" json:"name"`
	Descritpion string        `bson:"description" json:"description"`
	Status      string        `bson:"status" json:"status"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at"`
	DeletedAt   time.Time     `bson:"deleted_at,omitempty" json:"deleted_at"`
}
