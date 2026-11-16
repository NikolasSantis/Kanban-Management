package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Label struct {
	ID        *bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID *bson.ObjectID `bson:"project_id" json:"project_id"`
	BoardID   *bson.ObjectID `bson:"board_id" json:"board_id"`
	Name      *string        `bson:"name" json:"name"`
	// NameNormalized (minúsculo, sem espaços nas pontas) garante nome único por board.
	NameNormalized string  `bson:"name_normalized" json:"-"`
	Color          *string `bson:"color" json:"color"`
	// Active existe só para o índice único parcial: true enquanto a label não é
	// deletada. O Mongo não aceita {deleted_at: {$exists: false}} em
	// partialFilterExpression, então o soft delete também põe active=false.
	Active    bool           `bson:"active" json:"-"`
	CreatedBy *bson.ObjectID `bson:"created_by" json:"created_by"`
	CreatedAt time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time     `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}
