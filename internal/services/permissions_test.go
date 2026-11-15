package services

import "testing"

func TestCanMatrix(t *testing.T) {
	// true = permitido
	matrix := map[Action][3]bool{ // member, manager, admin
		ActionProjectView:            {true, true, true},
		ActionProjectUpdate:          {false, true, true},
		ActionProjectDelete:          {false, false, true},
		ActionProjectManageMembers:   {false, true, true},
		ActionBoardView:              {true, true, true},
		ActionBoardManage:            {false, true, true},
		ActionCardView:               {true, true, true},
		ActionCardCreate:             {true, true, true},
		ActionCardUpdate:             {true, true, true},
		ActionCardDelete:             {false, true, true},
		ActionLabelManage:            {false, true, true},
		ActionCommentCreate:          {true, true, true},
		ActionCommentModerate:        {false, true, true},
		ActionAttachmentCreate:       {true, true, true},
		ActionAttachmentModerate:     {false, true, true},
		ActionWorkspaceView:          {true, true, true},
		ActionWorkspaceUpdate:        {false, true, true},
		ActionWorkspaceDelete:        {false, false, true},
		ActionWorkspaceManageMembers: {false, true, true},
	}

	roles := []string{RoleMember, RoleManager, RoleAdmin}

	for action, expected := range matrix {
		for i, role := range roles {
			if got := Can(role, action); got != expected[i] {
				t.Errorf("Can(%q, %q) = %v, esperado %v", role, action, got, expected[i])
			}
		}
	}
}

func TestCanUnknownRole(t *testing.T) {
	if Can("", ActionProjectView) || Can("owner", ActionProjectView) {
		t.Error("role desconhecida não deve ter permissão")
	}
}
