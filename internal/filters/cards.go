// Package filters monta os filtros e a ordenação das listagens a partir da query
// string. A montagem é feita por funções PURAS (sem banco), para serem testadas
// com tabelas.
package filters

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"kanban-management/internal/models"
	"kanban-management/internal/search"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MaxValues é o máximo de valores (separados por vírgula) em um mesmo filtro.
const MaxValues = 20

const (
	StatusAll       = "all"
	StatusOpen      = "open"
	StatusCompleted = "completed"

	LabelModeAll = "all"
	LabelModeAny = "any"

	SortPosition  = "position"
	SortCreatedAt = "created_at"
	SortUpdatedAt = "updated_at"
	SortDueDate   = "due_date"
	SortPriority  = "priority"
	SortTitle     = "title"

	OrderAsc  = "asc"
	OrderDesc = "desc"
)

// QueryError é um erro de validação da query string (a API responde 400 com Message).
type QueryError struct {
	Message string
}

func (e *QueryError) Error() string { return e.Message }

func queryError(format string, args ...any) error {
	return &QueryError{Message: fmt.Sprintf(format, args...)}
}

// AsQueryError devolve o QueryError dentro de err, se houver.
func AsQueryError(err error) (*QueryError, bool) {
	var queryErr *QueryError

	if errors.As(err, &queryErr) {
		return queryErr, true
	}

	return nil, false
}

// CardQuery é a query de cards já validada.
type CardQuery struct {
	ColumnIDs    []string
	Priorities   []string
	LabelIDs     []bson.ObjectID
	LabelMode    string
	AssigneeID   *bson.ObjectID
	AssigneeNone bool
	CreatedBy    *bson.ObjectID
	Status       string
	DueBefore    *time.Time
	DueAfter     *time.Time
	Overdue      *bool
	HasDueDate   *bool
	Search       []string
	Sort         string
	Order        string
	// SortGiven: o cliente enviou sort (ou order) explicitamente.
	SortGiven bool

	Now         time.Time
	CurrentUser bson.ObjectID
}

// HasFilters diz se algum filtro, a busca ou uma ordenação explícita foi usado.
// Nesse caso a listagem de cards do board é uma lista plana (não agrupada por coluna).
func (q CardQuery) HasFilters() bool {
	return len(q.ColumnIDs) > 0 || len(q.Priorities) > 0 || len(q.LabelIDs) > 0 ||
		q.AssigneeID != nil || q.AssigneeNone || q.CreatedBy != nil ||
		(q.Status != "" && q.Status != StatusAll) ||
		q.DueBefore != nil || q.DueAfter != nil || q.Overdue != nil || q.HasDueDate != nil ||
		len(q.Search) > 0 || q.SortGiven
}

// SplitCSV separa "a,b,c" em valores: aplica trim, remove vazios e repetidos e
// recusa mais de max valores.
func SplitCSV(raw string, max int) ([]string, error) {
	values := []string{}
	seen := map[string]bool{}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)

		if part == "" || seen[part] {
			continue
		}

		seen[part] = true
		values = append(values, part)
	}

	if len(values) > max {
		return nil, errors.New("too many values")
	}

	return values, nil
}

func csv(get func(string) string, param string) ([]string, error) {
	values, err := SplitCSV(get(param), MaxValues)

	if err != nil {
		return nil, queryError("Too many values in %s (max %d)", param, MaxValues)
	}

	return values, nil
}

// ParseIDOrMe converte "me" no usuário logado ou valida um ObjectID.
func ParseIDOrMe(raw string, me bson.ObjectID) (bson.ObjectID, bool) {
	if raw == "me" {
		return me, true
	}

	id, err := bson.ObjectIDFromHex(raw)

	return id, err == nil
}

func parseBool(get func(string) string, param string) (*bool, error) {
	switch strings.ToLower(strings.TrimSpace(get(param))) {
	case "":
		return nil, nil
	case "true":
		value := true
		return &value, nil
	case "false":
		value := false
		return &value, nil
	default:
		return nil, queryError("Invalid %s. Allowed: true, false", param)
	}
}

// parseBoundary lê due_before/due_after. Só a data (YYYY-MM-DD): due_before vai
// até o fim do dia (23:59:59Z) e due_after começa no início do dia (00:00:00Z).
func parseBoundary(raw string, now time.Time, startOfDay bool) (time.Time, error) {
	t, err := models.ParseDueDateAt(raw, now)

	if err != nil {
		return time.Time{}, err
	}

	if startOfDay && len(strings.TrimSpace(raw)) == len("2006-01-02") {
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}

	return t, nil
}

// ParseCardQueryValues valida a query (get lê um parâmetro da query string).
func ParseCardQueryValues(get func(string) string, currentUser bson.ObjectID, now time.Time) (CardQuery, error) {
	q := CardQuery{
		LabelMode:   LabelModeAll,
		Status:      StatusAll,
		Sort:        SortPosition,
		Order:       OrderAsc,
		Now:         now,
		CurrentUser: currentUser,
	}

	var err error

	if q.ColumnIDs, err = csv(get, "column_id"); err != nil {
		return q, err
	}

	priorities, err := csv(get, "priority")

	if err != nil {
		return q, err
	}

	for _, raw := range priorities {
		priority, err := models.ParsePriority(raw)

		if err != nil {
			return q, queryError("%s", models.InvalidPriorityMessage)
		}

		q.Priorities = append(q.Priorities, string(priority))
	}

	labelIDs, err := csv(get, "label_ids")

	if err != nil {
		return q, err
	}

	for _, raw := range labelIDs {
		id, err := bson.ObjectIDFromHex(raw)

		if err != nil {
			return q, queryError("Invalid label_ids: %q is not a valid id", raw)
		}

		q.LabelIDs = append(q.LabelIDs, id)
	}

	if mode := strings.ToLower(strings.TrimSpace(get("label_mode"))); mode != "" {
		if mode != LabelModeAll && mode != LabelModeAny {
			return q, queryError("Invalid label_mode. Allowed: all, any")
		}

		q.LabelMode = mode
	}

	switch raw := strings.TrimSpace(get("assignee_id")); raw {
	case "":
	case "none":
		q.AssigneeNone = true
	default:
		id, ok := ParseIDOrMe(raw, currentUser)

		if !ok {
			return q, queryError("Invalid assignee_id. Use an id, me or none")
		}

		q.AssigneeID = &id
	}

	if raw := strings.TrimSpace(get("created_by")); raw != "" {
		id, ok := ParseIDOrMe(raw, currentUser)

		if !ok {
			return q, queryError("Invalid created_by. Use an id or me")
		}

		q.CreatedBy = &id
	}

	if status := strings.ToLower(strings.TrimSpace(get("status"))); status != "" {
		if status != StatusOpen && status != StatusCompleted && status != StatusAll {
			return q, queryError("Invalid status. Allowed: open, completed, all")
		}

		q.Status = status
	}

	if raw := strings.TrimSpace(get("due_before")); raw != "" {
		t, err := parseBoundary(raw, now, false)

		if err != nil {
			return q, queryError("%s", models.InvalidDueDateMessage)
		}

		q.DueBefore = &t
	}

	if raw := strings.TrimSpace(get("due_after")); raw != "" {
		t, err := parseBoundary(raw, now, true)

		if err != nil {
			return q, queryError("%s", models.InvalidDueDateMessage)
		}

		q.DueAfter = &t
	}

	if q.DueBefore != nil && q.DueAfter != nil && q.DueAfter.After(*q.DueBefore) {
		return q, queryError("due_after must not be after due_before")
	}

	if q.Overdue, err = parseBool(get, "overdue"); err != nil {
		return q, err
	}

	if q.HasDueDate, err = parseBool(get, "has_due_date"); err != nil {
		return q, err
	}

	if q.Overdue != nil && *q.Overdue && q.Status == StatusCompleted {
		return q, queryError("overdue=true cannot be combined with status=completed")
	}

	if q.HasDueDate != nil && !*q.HasDueDate && (q.DueBefore != nil || q.DueAfter != nil || (q.Overdue != nil && *q.Overdue)) {
		return q, queryError("has_due_date=false cannot be combined with due dates or overdue=true")
	}

	if raw := get("q"); strings.TrimSpace(raw) != "" {
		if q.Search, err = search.ParseQuery(raw); err != nil {
			return q, queryError("%s", search.InvalidQueryMessage)
		}
	}

	sortRaw := strings.ToLower(strings.TrimSpace(get("sort")))
	orderRaw := strings.ToLower(strings.TrimSpace(get("order")))

	if sortRaw != "" {
		switch sortRaw {
		case SortPosition, SortCreatedAt, SortUpdatedAt, SortDueDate, SortPriority, SortTitle:
			q.Sort = sortRaw
		default:
			return q, queryError("Invalid sort. Allowed: position, created_at, updated_at, due_date, priority, title")
		}

		q.SortGiven = true
	}

	if orderRaw != "" {
		if orderRaw != OrderAsc && orderRaw != OrderDesc {
			return q, queryError("Invalid order. Allowed: asc, desc")
		}

		q.Order = orderRaw
		q.SortGiven = true
	}

	// Com busca e sem ordenação explícita: os alterados mais recentemente primeiro.
	if len(q.Search) > 0 && sortRaw == "" && orderRaw == "" {
		q.Sort = SortUpdatedAt
		q.Order = OrderDesc
	}

	return q, nil
}

// ParseCardQuery lê a query string da request ("me" vira o usuário logado).
func ParseCardQuery(c *fiber.Ctx, currentUser bson.ObjectID, now time.Time) (CardQuery, error) {
	return ParseCardQueryValues(func(key string) string { return c.Query(key) }, currentUser, now)
}

// BuildCardFilter monta as condições dos filtros. É PURA. Não inclui o escopo
// (board_id/project_id) nem o soft delete: o handler acrescenta. Condições que
// precisam de "$or"/"$nor" ficam dentro de "$and", para nunca colidir com o
// "$or" do soft delete.
func BuildCardFilter(q CardQuery) bson.M {
	filter := bson.M{}
	and := []bson.M{}

	if len(q.ColumnIDs) > 0 {
		filter["column_id"] = bson.M{"$in": q.ColumnIDs}
	}

	if len(q.Priorities) > 0 {
		filter["priority"] = bson.M{"$in": q.Priorities}
	}

	if len(q.LabelIDs) > 0 {
		if q.LabelMode == LabelModeAny {
			filter["label_ids"] = bson.M{"$in": q.LabelIDs}
		} else {
			filter["label_ids"] = bson.M{"$all": q.LabelIDs}
		}
	}

	if q.AssigneeID != nil {
		filter["assignee_ids"] = *q.AssigneeID
	}

	if q.AssigneeNone {
		and = append(and, bson.M{"$or": []bson.M{
			{"assignee_ids": bson.M{"$exists": false}},
			{"assignee_ids": bson.M{"$size": 0}},
		}})
	}

	if q.CreatedBy != nil {
		filter["created_by"] = *q.CreatedBy
	}

	switch q.Status {
	case StatusOpen:
		filter["completed_at"] = bson.M{"$exists": false}
	case StatusCompleted:
		filter["completed_at"] = bson.M{"$exists": true}
	}

	due := bson.M{}

	if q.DueBefore != nil {
		due["$lte"] = *q.DueBefore
	}

	if q.DueAfter != nil {
		due["$gte"] = *q.DueAfter
	}

	if q.HasDueDate != nil {
		due["$exists"] = *q.HasDueDate
	}

	if q.Overdue != nil {
		if *q.Overdue {
			// Atrasado: tem data no passado e não foi concluído.
			due["$lt"] = q.Now
			filter["completed_at"] = bson.M{"$exists": false}
		} else {
			and = append(and, bson.M{"$nor": []bson.M{
				{"due_date": bson.M{"$lt": q.Now}, "completed_at": bson.M{"$exists": false}},
			}})
		}
	}

	if len(due) > 0 {
		filter["due_date"] = due
	}

	if len(q.Search) > 0 {
		and = append(and, search.RegexFilter("search_text", q.Search)["$and"].([]bson.M)...)
	}

	if len(and) > 0 {
		filter["$and"] = and
	}

	return filter
}

// BuildCardSort monta a ordenação (allowlist; desempate por _id na mesma direção).
// Para sort=due_date devolve o sort da aggregation (com o campo auxiliar has_due,
// que manda os cards sem data para o fim nos dois sentidos).
func BuildCardSort(q CardQuery) bson.D {
	direction := 1

	if q.Order == OrderDesc {
		direction = -1
	}

	field := "position"

	switch q.Sort {
	case SortCreatedAt:
		field = "created_at"
	case SortUpdatedAt:
		field = "updated_at"
	case SortDueDate:
		field = "due_date"
	case SortPriority:
		field = "priority_level"
	case SortTitle:
		field = "title"
	}

	sort := bson.D{}

	if q.Sort == SortDueDate {
		sort = append(sort, bson.E{Key: "has_due", Value: -1})
	}

	return append(sort, bson.E{Key: field, Value: direction}, bson.E{Key: "_id", Value: direction})
}

// NeedsAggregation: ordenar por due_date precisa do campo auxiliar has_due.
func (q CardQuery) NeedsAggregation() bool {
	return q.Sort == SortDueDate
}

// DueDateAddFields é o $addFields da aggregation do sort=due_date: has_due vale
// 1 quando o card tem data de entrega e 0 quando não tem (campo ausente ou nulo).
var DueDateAddFields = bson.M{"has_due": bson.M{"$cond": bson.A{
	bson.M{"$eq": bson.A{bson.M{"$type": "$due_date"}, "date"}}, 1, 0,
}}}
