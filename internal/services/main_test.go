package services_test

import (
	"os"
	"testing"

	"kanban-management/internal/tests/testutil"
)

func TestMain(m *testing.M) {
	code := m.Run()

	testutil.Cleanup()

	os.Exit(code)
}
