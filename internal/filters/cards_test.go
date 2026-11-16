package filters

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	now = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	me  = bson.NewObjectID()
	l1  = bson.NewObjectID()
	l2  = bson.NewObjectID()
	u1  = bson.NewObjectID()
)

func ptr[T any](v T) *T { return &v }

func TestBuildCardFilter(t *testing.T) {
	dayEnd := time.Date(2026, 12, 1, 23, 59, 59, 0, time.UTC)
	dayStart := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		query CardQuery
		want  bson.M
	}{
		{"sem filtros", CardQuery{}, bson.M{}},
		{"status all não filtra", CardQuery{Status: StatusAll}, bson.M{}},
		{"column_id", CardQuery{ColumnIDs: []string{"todo", "doing"}}, bson.M{
			"column_id": bson.M{"$in": []string{"todo", "doing"}},
		}},
		{"priority", CardQuery{Priorities: []string{"high", "urgent"}}, bson.M{
			"priority": bson.M{"$in": []string{"high", "urgent"}},
		}},
		{"label_ids: todas (padrão)", CardQuery{LabelIDs: []bson.ObjectID{l1, l2}}, bson.M{
			"label_ids": bson.M{"$all": []bson.ObjectID{l1, l2}},
		}},
		{"label_ids: all explícito", CardQuery{LabelIDs: []bson.ObjectID{l1}, LabelMode: LabelModeAll}, bson.M{
			"label_ids": bson.M{"$all": []bson.ObjectID{l1}},
		}},
		{"label_ids: any", CardQuery{LabelIDs: []bson.ObjectID{l1, l2}, LabelMode: LabelModeAny}, bson.M{
			"label_ids": bson.M{"$in": []bson.ObjectID{l1, l2}},
		}},
		{"assignee_id", CardQuery{AssigneeID: &u1}, bson.M{"assignee_ids": u1}},
		{"assignee_id=none", CardQuery{AssigneeNone: true}, bson.M{"$and": []bson.M{
			{"$or": []bson.M{
				{"assignee_ids": bson.M{"$exists": false}},
				{"assignee_ids": bson.M{"$size": 0}},
			}},
		}}},
		{"created_by", CardQuery{CreatedBy: &me}, bson.M{"created_by": me}},
		{"status=open", CardQuery{Status: StatusOpen}, bson.M{"completed_at": bson.M{"$exists": false}}},
		{"status=completed", CardQuery{Status: StatusCompleted}, bson.M{"completed_at": bson.M{"$exists": true}}},
		{"due_before", CardQuery{DueBefore: &dayEnd}, bson.M{"due_date": bson.M{"$lte": dayEnd}}},
		{"due_after", CardQuery{DueAfter: &dayStart}, bson.M{"due_date": bson.M{"$gte": dayStart}}},
		{"due_after + due_before no mesmo campo", CardQuery{DueAfter: &dayStart, DueBefore: &dayEnd}, bson.M{
			"due_date": bson.M{"$gte": dayStart, "$lte": dayEnd},
		}},
		{"has_due_date=true", CardQuery{HasDueDate: ptr(true)}, bson.M{"due_date": bson.M{"$exists": true}}},
		{"has_due_date=false", CardQuery{HasDueDate: ptr(false)}, bson.M{"due_date": bson.M{"$exists": false}}},
		{"overdue=true: data passada e não concluído", CardQuery{Overdue: ptr(true), Now: now}, bson.M{
			"due_date":     bson.M{"$lt": now},
			"completed_at": bson.M{"$exists": false},
		}},
		{"overdue=true + status=open não duplica nem conflita", CardQuery{Overdue: ptr(true), Status: StatusOpen, Now: now}, bson.M{
			"due_date":     bson.M{"$lt": now},
			"completed_at": bson.M{"$exists": false},
		}},
		{"overdue=false: o complemento", CardQuery{Overdue: ptr(false), Now: now}, bson.M{"$and": []bson.M{
			{"$nor": []bson.M{{"due_date": bson.M{"$lt": now}, "completed_at": bson.M{"$exists": false}}}},
		}}},
		{"busca: uma regex por palavra", CardQuery{Search: []string{"login", "impl"}}, bson.M{"$and": []bson.M{
			{"search_text": bson.M{"$regex": "login"}},
			{"search_text": bson.M{"$regex": "impl"}},
		}}},
		{"busca com metacaracteres é escapada", CardQuery{Search: []string{"a+b"}}, bson.M{"$and": []bson.M{
			{"search_text": bson.M{"$regex": `a\+b`}},
		}}},
		{"combinação: filtros simples + prazo", CardQuery{
			ColumnIDs: []string{"todo"}, Priorities: []string{"urgent"}, AssigneeID: &me, Status: StatusOpen, DueBefore: &dayEnd,
		}, bson.M{
			"column_id":    bson.M{"$in": []string{"todo"}},
			"priority":     bson.M{"$in": []string{"urgent"}},
			"assignee_ids": me,
			"completed_at": bson.M{"$exists": false},
			"due_date":     bson.M{"$lte": dayEnd},
		}},
		{"combinação: none + busca compartilham o mesmo $and", CardQuery{AssigneeNone: true, Search: []string{"login"}}, bson.M{"$and": []bson.M{
			{"$or": []bson.M{
				{"assignee_ids": bson.M{"$exists": false}},
				{"assignee_ids": bson.M{"$size": 0}},
			}},
			{"search_text": bson.M{"$regex": "login"}},
		}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, BuildCardFilter(tc.query))
		})
	}
}

func TestBuildCardFilter_NeverUsesTopLevelOrOrScopeKeys(t *testing.T) {
	filter := BuildCardFilter(CardQuery{AssigneeNone: true, Search: []string{"x"}, Overdue: ptr(false), Now: now})

	assert.NotContains(t, filter, "$or", "o $or do soft delete pertence ao handler")
	assert.NotContains(t, filter, "board_id")
	assert.NotContains(t, filter, "project_id")
	assert.NotContains(t, filter, "deleted_at")
}

func TestBuildCardSort(t *testing.T) {
	cases := []struct {
		name  string
		query CardQuery
		want  bson.D
	}{
		{"padrão: position asc", CardQuery{Sort: SortPosition, Order: OrderAsc}, bson.D{{Key: "position", Value: 1}, {Key: "_id", Value: 1}}},
		{"vazio cai em position asc", CardQuery{}, bson.D{{Key: "position", Value: 1}, {Key: "_id", Value: 1}}},
		{"position desc", CardQuery{Sort: SortPosition, Order: OrderDesc}, bson.D{{Key: "position", Value: -1}, {Key: "_id", Value: -1}}},
		{"created_at asc", CardQuery{Sort: SortCreatedAt, Order: OrderAsc}, bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}},
		{"created_at desc", CardQuery{Sort: SortCreatedAt, Order: OrderDesc}, bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}}},
		{"updated_at desc", CardQuery{Sort: SortUpdatedAt, Order: OrderDesc}, bson.D{{Key: "updated_at", Value: -1}, {Key: "_id", Value: -1}}},
		{"priority usa priority_level", CardQuery{Sort: SortPriority, Order: OrderDesc}, bson.D{{Key: "priority_level", Value: -1}, {Key: "_id", Value: -1}}},
		{"title asc", CardQuery{Sort: SortTitle, Order: OrderAsc}, bson.D{{Key: "title", Value: 1}, {Key: "_id", Value: 1}}},
		{"due_date asc: sem data no fim", CardQuery{Sort: SortDueDate, Order: OrderAsc}, bson.D{{Key: "has_due", Value: -1}, {Key: "due_date", Value: 1}, {Key: "_id", Value: 1}}},
		{"due_date desc: sem data continua no fim", CardQuery{Sort: SortDueDate, Order: OrderDesc}, bson.D{{Key: "has_due", Value: -1}, {Key: "due_date", Value: -1}, {Key: "_id", Value: -1}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, BuildCardSort(tc.query))
		})
	}
}

func TestNeedsAggregationOnlyForDueDate(t *testing.T) {
	assert.True(t, CardQuery{Sort: SortDueDate}.NeedsAggregation())

	for _, sort := range []string{SortPosition, SortCreatedAt, SortUpdatedAt, SortPriority, SortTitle, ""} {
		assert.False(t, CardQuery{Sort: sort}.NeedsAggregation(), sort)
	}
}

func values(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestParseCardQueryValues_Valid(t *testing.T) {
	t.Run("sem parâmetros: nenhum filtro e ordenação padrão", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(nil), me, now)

		require.NoError(t, err)
		assert.False(t, q.HasFilters())
		assert.Equal(t, SortPosition, q.Sort)
		assert.Equal(t, OrderAsc, q.Order)
		assert.Equal(t, StatusAll, q.Status)
		assert.Equal(t, LabelModeAll, q.LabelMode)
	})

	t.Run("me vira o usuário logado", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"assignee_id": "me", "created_by": "me"}), me, now)

		require.NoError(t, err)
		assert.Equal(t, me, *q.AssigneeID)
		assert.Equal(t, me, *q.CreatedBy)
		assert.True(t, q.HasFilters())
	})

	t.Run("assignee_id=none", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"assignee_id": "none"}), me, now)

		require.NoError(t, err)
		assert.True(t, q.AssigneeNone)
		assert.Nil(t, q.AssigneeID)
	})

	t.Run("listas: trim, sem vazios e sem repetidos; prioridade normalizada", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{
			"column_id": " todo , doing,,todo ", "priority": "HIGH, urgent",
			"label_ids": l1.Hex() + "," + l2.Hex(), "label_mode": "ANY",
		}), me, now)

		require.NoError(t, err)
		assert.Equal(t, []string{"todo", "doing"}, q.ColumnIDs)
		assert.Equal(t, []string{"high", "urgent"}, q.Priorities)
		assert.Equal(t, []bson.ObjectID{l1, l2}, q.LabelIDs)
		assert.Equal(t, LabelModeAny, q.LabelMode)
	})

	t.Run("datas: só a data usa fim do dia no due_before e início no due_after", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"due_before": "2026-12-01", "due_after": "2026-11-01"}), me, now)

		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 12, 1, 23, 59, 59, 0, time.UTC), *q.DueBefore)
		assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), *q.DueAfter)
	})

	t.Run("datas em RFC 3339 com fuso são convertidas para UTC", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"due_before": "2026-12-01T10:00:00-03:00", "due_after": "2026-12-01T10:00:00-03:00"}), me, now)

		require.NoError(t, err)
		assert.Equal(t, time.Date(2026, 12, 1, 13, 0, 0, 0, time.UTC), *q.DueBefore)
		assert.Equal(t, time.Date(2026, 12, 1, 13, 0, 0, 0, time.UTC), *q.DueAfter)
	})

	t.Run("booleanos", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"overdue": "TRUE", "has_due_date": "false"}), me, now)

		// has_due_date=false com overdue=true é contraditório
		assert.Error(t, err)

		q, err = ParseCardQueryValues(values(map[string]string{"overdue": "true", "has_due_date": "true"}), me, now)
		require.NoError(t, err)
		assert.True(t, *q.Overdue)
		assert.True(t, *q.HasDueDate)
	})

	t.Run("busca: normalizada; sem ordenação explícita ordena por updated_at desc", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"q": "  Implementação  LOGIN "}), me, now)

		require.NoError(t, err)
		assert.Equal(t, []string{"implementacao", "login"}, q.Search)
		assert.Equal(t, SortUpdatedAt, q.Sort)
		assert.Equal(t, OrderDesc, q.Order)
		assert.False(t, q.SortGiven)
	})

	t.Run("busca com ordenação explícita respeita o pedido", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"q": "login", "sort": "title", "order": "asc"}), me, now)

		require.NoError(t, err)
		assert.Equal(t, SortTitle, q.Sort)
		assert.Equal(t, OrderAsc, q.Order)
		assert.True(t, q.SortGiven)
	})

	t.Run("q só com espaços é a listagem normal", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"q": "   "}), me, now)

		require.NoError(t, err)
		assert.Empty(t, q.Search)
		assert.False(t, q.HasFilters())
	})

	t.Run("só sort já conta como filtro (lista plana)", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"sort": "due_date"}), me, now)

		require.NoError(t, err)
		assert.True(t, q.HasFilters())
	})

	t.Run("status=all não conta como filtro", func(t *testing.T) {
		q, err := ParseCardQueryValues(values(map[string]string{"status": "all"}), me, now)

		require.NoError(t, err)
		assert.False(t, q.HasFilters())
	})

	t.Run("20 valores no mesmo filtro é o máximo permitido", func(t *testing.T) {
		ids := make([]string, 20)
		for i := range ids {
			ids[i] = bson.NewObjectID().Hex()
		}

		_, err := ParseCardQueryValues(values(map[string]string{"label_ids": strings.Join(ids, ",")}), me, now)

		assert.NoError(t, err)
	})
}

func TestParseCardQueryValues_Errors(t *testing.T) {
	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = bson.NewObjectID().Hex()
	}

	cases := []struct {
		name    string
		params  map[string]string
		message string
	}{
		{"prioridade inválida lista os valores aceitos", map[string]string{"priority": "xyz"}, "Invalid priority. Allowed: low, medium, high, urgent"},
		{"uma prioridade inválida entre válidas", map[string]string{"priority": "high,xyz"}, "Invalid priority. Allowed: low, medium, high, urgent"},
		{"sort fora da allowlist", map[string]string{"sort": "password"}, "Invalid sort. Allowed: position, created_at, updated_at, due_date, priority, title"},
		{"order inválido", map[string]string{"order": "sideways"}, "Invalid order. Allowed: asc, desc"},
		{"label_ids com id inválido", map[string]string{"label_ids": "nao-e-id"}, `Invalid label_ids: "nao-e-id" is not a valid id`},
		{"label_mode inválido", map[string]string{"label_mode": "some"}, "Invalid label_mode. Allowed: all, any"},
		{"assignee_id inválido", map[string]string{"assignee_id": "xyz"}, "Invalid assignee_id. Use an id, me or none"},
		{"created_by inválido", map[string]string{"created_by": "none"}, "Invalid created_by. Use an id or me"},
		{"status inválido", map[string]string{"status": "done"}, "Invalid status. Allowed: open, completed, all"},
		{"due_before inválida", map[string]string{"due_before": "amanhã"}, "Invalid due_date. Use RFC 3339 (2026-12-01T18:00:00Z) or YYYY-MM-DD, between 2000 and 10 years ahead"},
		{"due_after fora do limite de anos", map[string]string{"due_after": "1990-01-01"}, "Invalid due_date. Use RFC 3339 (2026-12-01T18:00:00Z) or YYYY-MM-DD, between 2000 and 10 years ahead"},
		{"due_after depois de due_before", map[string]string{"due_after": "2026-12-10", "due_before": "2026-12-01"}, "due_after must not be after due_before"},
		{"overdue inválido", map[string]string{"overdue": "talvez"}, "Invalid overdue. Allowed: true, false"},
		{"has_due_date inválido", map[string]string{"has_due_date": "1"}, "Invalid has_due_date. Allowed: true, false"},
		{"overdue=true com status=completed", map[string]string{"overdue": "true", "status": "completed"}, "overdue=true cannot be combined with status=completed"},
		{"has_due_date=false com due_before", map[string]string{"has_due_date": "false", "due_before": "2026-12-01"}, "has_due_date=false cannot be combined with due dates or overdue=true"},
		{"mais de 20 valores", map[string]string{"label_ids": strings.Join(tooMany, ",")}, "Too many values in label_ids (max 20)"},
		{"mais de 20 colunas", map[string]string{"column_id": strings.Repeat("a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,", 1)}, "Too many values in column_id (max 20)"},
		{"q com 1 caractere", map[string]string{"q": "a"}, "Search term must have between 2 and 100 characters"},
		{"q com 101 caracteres", map[string]string{"q": strings.Repeat("a", 101)}, "Search term must have between 2 and 100 characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCardQueryValues(values(tc.params), me, now)

			require.Error(t, err)

			queryErr, ok := AsQueryError(err)
			require.True(t, ok, "o erro precisa ser um QueryError (vira 400)")
			assert.Equal(t, tc.message, queryErr.Message)
		})
	}
}

func TestSplitCSV(t *testing.T) {
	got, err := SplitCSV(" a , b,,a ,c ", 5)

	assert.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, got)

	got, err = SplitCSV("", 5)

	assert.NoError(t, err)
	assert.Empty(t, got)

	_, err = SplitCSV("a,b,c", 2)
	assert.Error(t, err)

	got, err = SplitCSV("a,a,a,a", 1)
	assert.NoError(t, err, "repetidos não contam para o limite")
	assert.Equal(t, []string{"a"}, got)
}

func TestParseIDOrMe(t *testing.T) {
	id, ok := ParseIDOrMe("me", me)
	assert.True(t, ok)
	assert.Equal(t, me, id)

	id, ok = ParseIDOrMe(u1.Hex(), me)
	assert.True(t, ok)
	assert.Equal(t, u1, id)

	_, ok = ParseIDOrMe("xyz", me)
	assert.False(t, ok)
}
