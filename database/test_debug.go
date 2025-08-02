package database

import (
	"testing"
)

func TestDebugConnection(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	t.Logf("TestDatabase created successfully with config: %+v", testDB.GetConfig())
}
