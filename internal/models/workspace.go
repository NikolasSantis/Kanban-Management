package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Workspace struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Description string        `bson:"description" json:"description"`
	OwnderID    bson.ObjectID `bson:"owner_id" json:"owner_id"`
	CreatedAt   time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updated_at"`
	DeletedAt   time.Time     `bson:"deleted_at, omitempty" json:"deleted_at"`
}
