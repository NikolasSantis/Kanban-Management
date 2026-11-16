package integration_test

import (
	"testing"

	"kanban-management/internal/tests/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteLabel_RemovesItFromCards(t *testing.T) {
	s := newCardScenario(t)
	bug := s.newLabel(t, "Bug")
	keep := s.newLabel(t, "Keep")

	first := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "todo", "um")
	second := testutil.CreateCard(t, s.app, s.admin.Token, s.boardID, "doing", "dois")

	for _, cardID := range []string{first, second} {
		for _, label := range []string{bug, keep} {
			status, _ := s.do(t, "POST", "/card/"+cardID+"/labels/"+label, s.admin.Token, nil)
			require.Equal(t, 200, status)
		}
	}

	status, _ := s.do(t, "DELETE", "/label/"+bug, s.admin.Token, nil)
	require.Equal(t, 200, status)

	for _, cardID := range []string{first, second} {
		assert.Equal(t, []string{keep}, labelIDs(s.cardJSON(t, cardID)), "a label deletada sai dos cards, a outra fica")
	}

	status, _ = s.do(t, "POST", "/card/"+first+"/labels/"+bug, s.admin.Token, nil)
	assert.Equal(t, 400, status, "label deletada não pode ser aplicada")
}
