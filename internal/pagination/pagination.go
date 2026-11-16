// Package pagination implementa a paginação por página (page + limit) usada
// por todas as rotas de listagem.
package pagination

import (
	"context"
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
	// MaxPage evita overflow em (page-1)*limit com valores absurdos.
	MaxPage = 1_000_000
)

// ErrInvalidParams: page ou limit não numéricos (a API responde 400).
var ErrInvalidParams = errors.New("invalid pagination params")

// InvalidParamsMessage é a mensagem de 400 devolvida pela API.
const InvalidParamsMessage = "Invalid pagination params"

type Params struct {
	Page  int
	Limit int
}

// Skip é quantos itens pular no Mongo.
func (p Params) Skip() int64 {
	return int64(p.Page-1) * int64(p.Limit)
}

// ParseValues aplica defaults e limites a page/limit em texto (vazio = padrão).
// page < 1 vira 1; limit < 1 vira 1; limit > 100 vira 100; não numérico é erro.
func ParseValues(pageRaw, limitRaw string) (Params, error) {
	params := Params{Page: DefaultPage, Limit: DefaultLimit}

	if pageRaw != "" {
		page, err := strconv.Atoi(pageRaw)

		if err != nil {
			return Params{}, ErrInvalidParams
		}

		params.Page = min(max(page, 1), MaxPage)
	}

	if limitRaw != "" {
		limit, err := strconv.Atoi(limitRaw)

		if err != nil {
			return Params{}, ErrInvalidParams
		}

		params.Limit = min(max(limit, 1), MaxLimit)
	}

	return params, nil
}

// Parse lê ?page= e ?limit= da request.
func Parse(c *fiber.Ctx) (Params, error) {
	return ParseValues(c.Query("page"), c.Query("limit"))
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
}

func NewMeta(p Params, total int64) Meta {
	totalPages := int((total + int64(p.Limit) - 1) / int64(p.Limit))

	return Meta{
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    p.Page < totalPages,
	}
}

type Page[T any] struct {
	Items      []T  `json:"items"`
	Pagination Meta `json:"pagination"`
}

// NewPage monta a resposta; items nil vira [] (nunca null).
func NewPage[T any](items []T, p Params, total int64) Page[T] {
	if items == nil {
		items = []T{}
	}

	return Page[T]{Items: items, Pagination: NewMeta(p, total)}
}

// FindOptions monta skip, limit e sort do Find.
func FindOptions(p Params, sort bson.D) *options.FindOptionsBuilder {
	return options.Find().SetSkip(p.Skip()).SetLimit(int64(p.Limit)).SetSort(sort)
}

// Fetch conta (mesmo filtro) e busca a página: duas queries.
func Fetch[T any](ctx context.Context, collection *mongo.Collection, filter bson.M, sort bson.D, p Params) ([]T, int64, error) {
	total, err := collection.CountDocuments(ctx, filter)

	if err != nil {
		return nil, 0, err
	}

	items := []T{}

	if total == 0 || p.Skip() >= total {
		return items, total, nil
	}

	cursor, err := collection.Find(ctx, filter, FindOptions(p, sort))

	if err != nil {
		return nil, 0, err
	}

	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// FetchPipeline é o Fetch com uma aggregation ($match → $addFields → $sort →
// $skip → $limit). Usado quando a ordenação precisa de um campo calculado
// (ex.: cards sem data de entrega no fim). O count usa o mesmo filtro.
func FetchPipeline[T any](ctx context.Context, collection *mongo.Collection, filter bson.M, addFields bson.M, sort bson.D, p Params) ([]T, int64, error) {
	total, err := collection.CountDocuments(ctx, filter)

	if err != nil {
		return nil, 0, err
	}

	items := []T{}

	if total == 0 || p.Skip() >= total {
		return items, total, nil
	}

	cursor, err := collection.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$addFields", Value: addFields}},
		{{Key: "$sort", Value: sort}},
		{{Key: "$skip", Value: p.Skip()}},
		{{Key: "$limit", Value: int64(p.Limit)}},
	})

	if err != nil {
		return nil, 0, err
	}

	defer cursor.Close(ctx)

	if err := cursor.All(ctx, &items); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
