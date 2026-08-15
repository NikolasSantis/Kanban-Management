package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type User struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name            string        `bson:"name" json:"name"`
	Email           string        `bson:"email" json:"email"`
	EmailVerifiedAt time.Time     `bson:"email_verified_at,omitempty" json:"email_verified_at"`
	Password        string        `bson:"password" json:"password"`
	ExpirationDate  time.Time     `bson:"expiration_time" json:"expiration_date"`
	CreatedAt       time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt       time.Time     `bson:"updated_at" json:"updated_at"`
	DeletedAt       time.Time     `bson:"deleted_at,omitempty" json:"deleted_at"`
}
