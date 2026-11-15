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

func TestCanChangeMemberRole(t *testing.T) {
	cases := []struct {
		actor, current, next string
		want                 bool
	}{
		{RoleManager, RoleMember, RoleManager, true},
		{RoleManager, RoleManager, RoleMember, true},
		{RoleManager, RoleMember, RoleAdmin, false},
		{RoleManager, RoleAdmin, RoleMember, false},
		{RoleManager, RoleAdmin, "", false},
		{RoleManager, RoleMember, "", true},
		{RoleAdmin, RoleMember, RoleAdmin, true},
		{RoleAdmin, RoleAdmin, RoleMember, true},
		{RoleAdmin, RoleAdmin, "", true},
	}

	for _, tc := range cases {
		if got := CanChangeMemberRole(tc.actor, tc.current, tc.next); got != tc.want {
			t.Errorf("CanChangeMemberRole(%q,%q,%q) = %v, esperado %v", tc.actor, tc.current, tc.next, got, tc.want)
		}
	}
}
