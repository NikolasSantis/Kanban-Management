package services

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"kanban-management/internal/database"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	// PositionStep é o espaçamento entre cards de uma coluna.
	PositionStep = 1000.0
	// MinPositionGap: abaixo disso a coluna é renumerada.
	MinPositionGap = 1e-6

	MaxCardTitleLength       = 200
	MaxCardDescriptionLength = 5000
	MaxCardAssignees         = 10
)

var (
	// ErrInvalidAssignees: algum responsável não é membro ativo do projeto.
	ErrInvalidAssignees = errors.New("assignees must be active project members")
	// ErrInvalidCard: validação de campos do card (a mensagem é segura para o cliente).
	ErrInvalidCard = errors.New("invalid card")
)

// FindActiveCard busca um card que não foi deletado (soft delete).
func FindActiveCard(ctx context.Context, cardID bson.ObjectID) (*models.Card, error) {
	var card models.Card

	err := database.GetCollection("cards").FindOne(ctx, bson.M{
		"_id": cardID,
		"$or": NotDeleted(),
	}).Decode(&card)

	if err != nil {
		return nil, err
	}

	return &card, nil
}

// ColumnExists diz se o board tem uma coluna não deletada com esse id.
func ColumnExists(board *models.Board, columnID string) bool {
	if board == nil || columnID == "" {
		return false
	}

	for _, column := range board.Columns {
		if column.ID != nil && *column.ID == columnID && column.DeletedAt == nil {
			return true
		}
	}

	return false
}

// PositionBetween calcula a position de um card que ficará entre before
// (o card imediatamente acima) e after (o imediatamente abaixo). Qualquer um
// dos dois pode ser nil: sem nenhum, é o primeiro da coluna.
func PositionBetween(before, after *float64) float64 {
	switch {
	case before == nil && after == nil:
		return PositionStep
	case after == nil:
		return *before + PositionStep
	case before == nil:
		return *after / 2
	default:
		return (*before + *after) / 2
	}
}

// NextPosition devolve a position do fim da coluna: maior position + 1000
// (ou 1000 se a coluna está vazia).
func NextPosition(ctx context.Context, boardID bson.ObjectID, columnID string) (float64, error) {
	last, err := lastCardPosition(ctx, bson.M{
		"board_id":  boardID,
		"column_id": columnID,
		"$or":       NotDeleted(),
	})

	if err != nil {
		return 0, err
	}

	return PositionBetween(last, nil), nil
}

// NeighborPosition procura a position do vizinho de pivot na coluna, ignorando
// o card movido. next=true devolve o vizinho de baixo; false, o de cima.
// Devolve nil se não houver vizinho.
func NeighborPosition(ctx context.Context, boardID bson.ObjectID, columnID string, pivot float64, excludeID bson.ObjectID, next bool) (*float64, error) {
	filter := bson.M{
		"board_id":  boardID,
		"column_id": columnID,
		"_id":       bson.M{"$ne": excludeID},
		"$or":       NotDeleted(),
	}

	sortDir := -1

	if next {
		filter["position"] = bson.M{"$gt": pivot}
		sortDir = 1
	} else {
		filter["position"] = bson.M{"$lt": pivot}
	}

	var card models.Card

	err := database.GetCollection("cards").FindOne(ctx, filter,
		options.FindOne().SetSort(bson.D{{Key: "position", Value: sortDir}}),
	).Decode(&card)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &card.Position, nil
}

// LastPositionExcluding devolve a maior position da coluna ignorando um card.
func LastPositionExcluding(ctx context.Context, boardID bson.ObjectID, columnID string, excludeID bson.ObjectID) (*float64, error) {
	return lastCardPosition(ctx, bson.M{
		"board_id":  boardID,
		"column_id": columnID,
		"_id":       bson.M{"$ne": excludeID},
		"$or":       NotDeleted(),
	})
}

func lastCardPosition(ctx context.Context, filter bson.M) (*float64, error) {
	var card models.Card

	err := database.GetCollection("cards").FindOne(ctx, filter,
		options.FindOne().SetSort(bson.D{{Key: "position", Value: -1}}),
	).Decode(&card)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &card.Position, nil
}

// RenumberColumn reatribui 1000, 2000, 3000... aos cards ativos da coluna,
// mantendo a ordem atual. Usado quando o espaço entre dois cards acaba.
func RenumberColumn(ctx context.Context, boardID bson.ObjectID, columnID string) error {
	collection := database.GetCollection("cards")

	cursor, err := collection.Find(ctx, bson.M{
		"board_id":  boardID,
		"column_id": columnID,
		"$or":       NotDeleted(),
	}, options.Find().SetSort(bson.D{{Key: "position", Value: 1}, {Key: "_id", Value: 1}}))

	if err != nil {
		return err
	}

	defer cursor.Close(ctx)

	var cards []models.Card

	if err := cursor.All(ctx, &cards); err != nil {
		return err
	}

	if len(cards) == 0 {
		return nil
	}

	writes := make([]mongo.WriteModel, 0, len(cards))

	for i, card := range cards {
		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": card.ID}).
			SetUpdate(bson.M{"$set": bson.M{"position": float64(i+1) * PositionStep}}))
	}

	_, err = collection.BulkWrite(ctx, writes)

	return err
}

// ValidateAssignees exige que todos os ids sejam membros ativos do projeto.
// Duplicados e mais de MaxCardAssignees também são erro.
func ValidateAssignees(ctx context.Context, projectID bson.ObjectID, ids []bson.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}

	if len(ids) > MaxCardAssignees {
		return ErrInvalidAssignees
	}

	seen := make(map[bson.ObjectID]bool, len(ids))

	for _, id := range ids {
		if seen[id] {
			return ErrInvalidAssignees
		}

		seen[id] = true
	}

	count, err := database.GetCollection("project_members").CountDocuments(ctx, bson.M{
		"project_id": projectID,
		"user_id":    bson.M{"$in": ids},
		"$or":        NotDeleted(),
	})

	if err != nil {
		return err
	}

	if int(count) != len(ids) {
		return ErrInvalidAssignees
	}

	return nil
}

// NormalizeCardTitle aplica TrimSpace e valida 1–200 caracteres.
func NormalizeCardTitle(title string) (string, error) {
	title = strings.TrimSpace(title)

	if length := utf8.RuneCountInString(title); length < 1 || length > MaxCardTitleLength {
		return "", ErrInvalidCard
	}

	return title, nil
}

// ValidateCardDescription limita a descrição a 5000 caracteres.
func ValidateCardDescription(description string) error {
	if utf8.RuneCountInString(description) > MaxCardDescriptionLength {
		return ErrInvalidCard
	}

	return nil
}

// ActiveCardCount conta os cards ativos de uma coluna.
func ActiveCardCount(ctx context.Context, boardID bson.ObjectID, columnID string) (int64, error) {
	return database.GetCollection("cards").CountDocuments(ctx, bson.M{
		"board_id":  boardID,
		"column_id": columnID,
		"$or":       NotDeleted(),
	})
}
