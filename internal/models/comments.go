package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Comment struct {
	ID        *bson.ObjectID `bson:"_id,omitempty" json:"id"`
	ProjectID *bson.ObjectID `bson:"project_id" json:"project_id"`
	BoardID   *bson.ObjectID `bson:"board_id" json:"board_id"`
	CardID    *bson.ObjectID `bson:"card_id" json:"card_id"`
	AuthorID  *bson.ObjectID `bson:"author_id" json:"author_id"`
	Content   *string        `bson:"content" json:"content"`
	EditedAt  *time.Time     `bson:"edited_at,omitempty" json:"edited_at,omitempty"`
	CreatedAt time.Time      `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at" json:"updated_at"`
	DeletedAt *time.Time     `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy *bson.ObjectID `bson:"deleted_by,omitempty" json:"-"`
}

// RemovedUserName é o nome mostrado quando o autor do comentário foi deletado.
const RemovedUserName = "Usuário removido"

// CommentAuthor traz só o mínimo do autor (nunca e-mail nem senha).
type CommentAuthor struct {
	ID   bson.ObjectID `json:"id"`
	Name string        `json:"name"`
}

type CommentResponse struct {
	Comment
	Author *CommentAuthor `json:"author"`
}
