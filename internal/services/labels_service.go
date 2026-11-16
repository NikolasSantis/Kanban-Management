package services

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"kanban-management/internal/database"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	MaxLabelNameLength = 30
	MaxLabelsPerBoard  = 50
	MaxLabelsPerCard   = 10
)

var (
	// ErrInvalidLabelName: nome fora de 1–30 caracteres (após TrimSpace).
	ErrInvalidLabelName = errors.New("invalid label name")
	// ErrInvalidLabels: label inexistente, deletada, de outro board, duplicada ou acima do limite.
	ErrInvalidLabels = errors.New("invalid labels")
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// LabelPalette é a paleta usada quando o cliente não envia cor.
var LabelPalette = []string{"#61bd4f", "#f2d600", "#ff9f1a", "#eb5a46", "#c377e0", "#0079bf", "#00c2e0", "#51e898"}

// NormalizeLabelName devolve o nome sem espaços nas pontas e em minúsculas
// (a chave da unicidade por board).
func NormalizeLabelName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// CleanLabelName aplica TrimSpace e valida 1–30 caracteres.
func CleanLabelName(s string) (string, error) {
	s = strings.TrimSpace(s)

	if length := utf8.RuneCountInString(s); length < 1 || length > MaxLabelNameLength {
		return "", ErrInvalidLabelName
	}

	return s, nil
}

// ValidateColor aceita só #RRGGBB.
func ValidateColor(s string) bool {
	return colorPattern.MatchString(s)
}

// PaletteColor escolhe uma cor da paleta de forma determinística pelo índice.
func PaletteColor(index int) string {
	if index < 0 {
		index = -index
	}

	return LabelPalette[index%len(LabelPalette)]
}

// FindActiveLabel busca uma label que não foi deletada.
func FindActiveLabel(ctx context.Context, labelID bson.ObjectID) (*models.Label, error) {
	var label models.Label

	err := database.GetCollection("labels").FindOne(ctx, bson.M{
		"_id": labelID,
		"$or": NotDeleted(),
	}).Decode(&label)

	if err != nil {
		return nil, err
	}

	return &label, nil
}

// ValidateLabelIDs exige que todas as labels existam, não estejam deletadas e
// sejam do board; sem duplicadas e no máximo MaxLabelsPerCard.
func ValidateLabelIDs(ctx context.Context, boardID bson.ObjectID, ids []bson.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}

	if len(ids) > MaxLabelsPerCard {
		return ErrInvalidLabels
	}

	seen := make(map[bson.ObjectID]bool, len(ids))

	for _, id := range ids {
		if seen[id] {
			return ErrInvalidLabels
		}

		seen[id] = true
	}

	count, err := database.GetCollection("labels").CountDocuments(ctx, bson.M{
		"_id":      bson.M{"$in": ids},
		"board_id": boardID,
		"$or":      NotDeleted(),
	})

	if err != nil {
		return err
	}

	if int(count) != len(ids) {
		return ErrInvalidLabels
	}

	return nil
}

// ActiveLabelCount conta as labels ativas do board.
func ActiveLabelCount(ctx context.Context, boardID bson.ObjectID) (int64, error) {
	return database.GetCollection("labels").CountDocuments(ctx, bson.M{
		"board_id": boardID,
		"$or":      NotDeleted(),
	})
}
