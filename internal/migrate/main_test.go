package migrate_test

import (
	"os"
	"testing"

	"go.e64ec.com/booksmk/internal/testdb"
)

func TestMain(m *testing.M) {
	code := m.Run()
	testdb.Stop()
	os.Exit(code)
}
