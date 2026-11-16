package services

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"kanban-management/internal/database"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const MaxCommentLength = 5000

// ErrInvalidComment: texto vazio (após TrimSpace) ou com mais de 5000 caracteres.
var ErrInvalidComment = errors.New("invalid comment")

// ValidateCommentContent aplica TrimSpace e valida 1–5000 caracteres.
func ValidateCommentContent(s string) (string, error) {
	s = strings.TrimSpace(s)

	if length := utf8.RuneCountInString(s); length < 1 || length > MaxCommentLength {
		return "", ErrInvalidComment
	}

	return s, nil
}

// FindActiveComment busca um comentário que não foi deletado.
func FindActiveComment(ctx context.Context, commentID bson.ObjectID) (*models.Comment, error) {
	var comment models.Comment

	err := database.GetCollection("comments").FindOne(ctx, bson.M{
		"_id": commentID,
		"$or": NotDeleted(),
	}).Decode(&comment)

	if err != nil {
		return nil, err
	}

	return &comment, nil
}

// LoadAuthors busca nome dos autores ATIVOS com UMA query ($in, projeção
// {_id, name}). Quem não estiver no mapa foi deletado (ou não existe).
func LoadAuthors(ctx context.Context, ids []bson.ObjectID) (map[bson.ObjectID]models.CommentAuthor, error) {
	authors := make(map[bson.ObjectID]models.CommentAuthor, len(ids))

	if len(ids) == 0 {
		return authors, nil
	}

	cursor, err := database.GetCollection("users").Find(ctx, bson.M{
		"_id": bson.M{"$in": ids},
		"$or": NotDeleted(),
	}, options.Find().SetProjection(bson.M{"_id": 1, "name": 1}))

	if err != nil {
		return nil, err
	}

	defer cursor.Close(ctx)

	var users []struct {
		ID   bson.ObjectID `bson:"_id"`
		Name string        `bson:"name"`
	}

	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}

	for _, user := range users {
		authors[user.ID] = models.CommentAuthor{ID: user.ID, Name: user.Name}
	}

	return authors, nil
}

// ToCommentResponses monta a resposta com o autor de cada comentário. Autor
// deletado (ausente em authors) vira {id, "Usuário removido"}.
func ToCommentResponses(comments []models.Comment, authors map[bson.ObjectID]models.CommentAuthor) []models.CommentResponse {
	responses := make([]models.CommentResponse, 0, len(comments))

	for _, comment := range comments {
		response := models.CommentResponse{Comment: comment}

		if comment.AuthorID != nil {
			author, ok := authors[*comment.AuthorID]

			if !ok {
				author = models.CommentAuthor{ID: *comment.AuthorID, Name: models.RemovedUserName}
			}

			response.Author = &author
		}

		responses = append(responses, response)
	}

	return responses
}
