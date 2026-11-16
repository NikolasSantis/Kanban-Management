package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Board struct {
	ID          *bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID   *bson.ObjectID `bson:"project_id" json:"project_id"`
	Name        *string        `bson:"name" json:"name"`
	Description *string        `bson:"description" json:"description"`
	Columns     []Column       `bson:"columns" json:"columns"`
	Members     []Member       `bson:"members" json:"members"`
	Color       *string        `bson:"color" json:"color"`
	Background  *string        `bson:"background" json:"background"`
	CreatedAt   time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time      `bson:"updated_at" json:"updated_at"`
	// SearchText: nome normalizado (sem acentos, minúsculas); só para busca.
	SearchText string     `bson:"search_text" json:"-"`
	DeletedAt  *time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

type Column struct {
	ID        *string    `bson:"id" json:"id"`
	Name      *string    `bson:"name" json:"name"`
	Position  *int32     `bson:"position" json:"position"`
	Color     *string    `bson:"color" json:"color"`
	Icon      *string    `bson:"icon" json:"icon"`
	CreatedAt time.Time  `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time  `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}

type Member struct {
	ID        *bson.ObjectID `bson:"id" json:"id"`
	UserID    *bson.ObjectID `bson:"user_id" json:"user_id"`
	Name      *string        `bson:"name" json:"name"`
	Role      *string        `bson:"role" json:"role"`
	CreatedAt time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time     `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
}
