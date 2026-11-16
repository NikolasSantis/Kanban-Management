package testutil

import "testing"

func TestAssertTestDatabase(t *testing.T) {
	cases := []struct {
		name     string
		main, db string
		wantErr  bool
	}{
		{"ok", "kanban_management", "kanban_management_test", false},
		{"igual ao principal", "kanban_test", "kanban_test", true},
		{"sem sufixo _test", "kanban", "kanban_dev", true},
		{"vazio", "kanban", "", true},
		{"principal sem sufixo", "kanban_management", "kanban_management", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := AssertTestDatabase(tc.main, tc.db); (err != nil) != tc.wantErr {
				t.Errorf("AssertTestDatabase(%q,%q) erro=%v, esperava erro=%v", tc.main, tc.db, err, tc.wantErr)
			}
		})
	}
}
