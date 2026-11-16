package integration_test

import (
	"os"
	"testing"

	"kanban-management/internal/tests/testutil"
)

// TestMain roda a suíte e, no final, apaga o banco de teste (somente ele:
// testutil.DropTestDatabase recusa qualquer banco que não termine em _test).
func TestMain(m *testing.M) {
	code := m.Run()

	testutil.Cleanup()

	os.Exit(code)
}
