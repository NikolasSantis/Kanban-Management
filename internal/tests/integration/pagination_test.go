package integration_test

import (
	"fmt"
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagination_Boards25ItemsIn3Pages(t *testing.T) {
	s := newBoardScenario(t)

	for i := 0; i < 25; i++ {
		testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)
	}

	path := "/board/project/" + s.projectID
	seen := map[string]bool{}
	wantSizes := []int{10, 10, 5}

	for page, wantSize := range wantSizes {
		status, env := s.do(t, "GET", fmt.Sprintf("%s?limit=10&page=%d", path, page+1), s.admin.Token, nil)

		require.Equal(t, 200, status)
		require.Len(t, env.List(), wantSize, "página %d", page+1)

		meta := env.Pagination()
		assert.Equal(t, float64(page+1), meta["page"])
		assert.Equal(t, float64(10), meta["limit"])
		assert.Equal(t, float64(25), meta["total"])
		assert.Equal(t, float64(3), meta["total_pages"])
		assert.Equal(t, page < 2, meta["has_next"])

		for _, board := range env.List() {
			id := board["id"].(string)
			assert.False(t, seen[id], "o board %s apareceu em duas páginas", id)
			seen[id] = true
		}
	}

	assert.Len(t, seen, 25, "percorrer todas as páginas devolve exatamente o total, sem duplicados")

	t.Run("página além do fim devolve items vazio", func(t *testing.T) {
		status, env := s.do(t, "GET", path+"?limit=10&page=4", s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.JSONEq(t, "[]", string(env.ItemsRaw()))
		assert.Equal(t, float64(25), env.Pagination()["total"])
		assert.Equal(t, false, env.Pagination()["has_next"])
	})
}

func TestPagination_QueryParams(t *testing.T) {
	s := newBoardScenario(t)
	testutil.CreateBoard(t, s.app, s.admin.Token, s.projectID)
	path := "/board/project/" + s.projectID

	t.Run("padrões: page 1 e limit 20", func(t *testing.T) {
		_, env := s.do(t, "GET", path, s.admin.Token, nil)

		assert.Equal(t, float64(1), env.Pagination()["page"])
		assert.Equal(t, float64(20), env.Pagination()["limit"])
	})

	t.Run("limit=500 vira 100", func(t *testing.T) {
		status, env := s.do(t, "GET", path+"?limit=500", s.admin.Token, nil)

		require.Equal(t, 200, status)
		assert.Equal(t, float64(100), env.Pagination()["limit"])
	})

	t.Run("page=-1 vira 1 e page=0 vira 1", func(t *testing.T) {
		for _, page := range []string{"-1", "0"} {
			status, env := s.do(t, "GET", path+"?page="+page, s.admin.Token, nil)

			require.Equal(t, 200, status)
			assert.Equal(t, float64(1), env.Pagination()["page"])
			assert.Len(t, env.List(), 1)
		}
	})

	t.Run("valores não numéricos devolvem 400", func(t *testing.T) {
		for _, query := range []string{"?page=abc", "?limit=abc", "?page=1.5", "?limit="} {
			status, env := s.do(t, "GET", path+query, s.admin.Token, nil)

			if query == "?limit=" {
				assert.Equal(t, 200, status, "limit vazio usa o padrão")
				continue
			}

			assert.Equal(t, 400, status, query)
			assert.Equal(t, "Invalid pagination params", *env.Error)
		}
	})

	t.Run("permissão vem antes: não-membro recebe 404 mesmo com params inválidos", func(t *testing.T) {
		status, _ := s.do(t, "GET", path+"?page=abc", s.outsider.Token, nil)

		assert.Equal(t, 404, status)
	})
}
