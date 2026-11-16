package pagination

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseValues(t *testing.T) {
	cases := []struct {
		name      string
		page      string
		limit     string
		want      Params
		wantError bool
	}{
		{"sem parâmetros usa os padrões", "", "", Params{1, 20}, false},
		{"valores válidos", "3", "50", Params{3, 50}, false},
		{"page=0 vira 1", "0", "", Params{1, 20}, false},
		{"page negativa vira 1", "-1", "", Params{1, 20}, false},
		{"limit=1000 vira 100", "", "1000", Params{1, 100}, false},
		{"limit=500 vira 100", "", "500", Params{1, 100}, false},
		{"limit=0 vira 1", "", "0", Params{1, 1}, false},
		{"limit negativo vira 1", "", "-5", Params{1, 1}, false},
		{"limit=100 é permitido", "", "100", Params{1, 100}, false},
		{"page absurda é limitada", "99999999999", "", Params{MaxPage, 20}, false},
		{"limit não numérico", "", "abc", Params{}, true},
		{"page não numérica", "abc", "", Params{}, true},
		{"page decimal", "1.5", "", Params{}, true},
		{"page gigante demais para int", "99999999999999999999999", "", Params{}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseValues(tc.page, tc.limit)

			if tc.wantError {
				assert.ErrorIs(t, err, ErrInvalidParams)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSkip(t *testing.T) {
	assert.Equal(t, int64(0), Params{Page: 1, Limit: 20}.Skip())
	assert.Equal(t, int64(20), Params{Page: 2, Limit: 20}.Skip())
	assert.Equal(t, int64(90), Params{Page: 10, Limit: 10}.Skip())
	assert.Equal(t, int64((MaxPage-1)*100), Params{Page: MaxPage, Limit: 100}.Skip(), "sem overflow")
}

func TestNewMeta(t *testing.T) {
	t.Run("total 0", func(t *testing.T) {
		meta := NewMeta(Params{1, 20}, 0)

		assert.Equal(t, 0, meta.TotalPages)
		assert.False(t, meta.HasNext)
	})

	t.Run("57 itens com limit 20 são 3 páginas", func(t *testing.T) {
		assert.Equal(t, 3, NewMeta(Params{1, 20}, 57).TotalPages)
		assert.True(t, NewMeta(Params{1, 20}, 57).HasNext)
		assert.True(t, NewMeta(Params{2, 20}, 57).HasNext)
		assert.False(t, NewMeta(Params{3, 20}, 57).HasNext)
	})

	t.Run("total exato é múltiplo do limit", func(t *testing.T) {
		meta := NewMeta(Params{2, 20}, 40)

		assert.Equal(t, 2, meta.TotalPages)
		assert.False(t, meta.HasNext)
	})

	t.Run("página além do fim", func(t *testing.T) {
		meta := NewMeta(Params{9, 20}, 57)

		assert.Equal(t, 3, meta.TotalPages)
		assert.False(t, meta.HasNext)
	})
}

func TestNewPage_ItemsAreNeverNull(t *testing.T) {
	page := NewPage[string](nil, Params{1, 20}, 0)

	raw, err := json.Marshal(page)
	require.NoError(t, err)

	assert.JSONEq(t, `{"items":[],"pagination":{"page":1,"limit":20,"total":0,"total_pages":0,"has_next":false}}`, string(raw))
}

func TestNewPage_KeepsItems(t *testing.T) {
	page := NewPage([]int{1, 2}, Params{1, 2}, 5)

	assert.Equal(t, []int{1, 2}, page.Items)
	assert.Equal(t, int64(5), page.Pagination.Total)
	assert.Equal(t, 3, page.Pagination.TotalPages)
	assert.True(t, page.Pagination.HasNext)
}

func TestFindOptions(t *testing.T) {
	opts := FindOptions(Params{3, 10}, nil)

	assert.NotNil(t, opts)
}
