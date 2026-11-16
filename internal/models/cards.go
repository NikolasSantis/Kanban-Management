package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Card struct {
	ID          *bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	ProjectID   *bson.ObjectID  `bson:"project_id" json:"project_id"`
	BoardID     *bson.ObjectID  `bson:"board_id" json:"board_id"`
	ColumnID    *string         `bson:"column_id" json:"column_id"`
	Title       *string         `bson:"title" json:"title"`
	Description *string         `bson:"description" json:"description"`
	Position    float64         `bson:"position" json:"position"`
	AssigneeIDs []bson.ObjectID `bson:"assignee_ids" json:"assignee_ids"`
	CreatedBy   *bson.ObjectID  `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time       `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time       `bson:"updated_at" json:"updated_at"`
	DeletedAt   *time.Time      `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}
