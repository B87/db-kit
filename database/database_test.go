package database

import (
	"os"
	"testing"
)

// TestMain is the entry point for the test suite of the package.
func TestMain(m *testing.M) {
	// Run tests - expects a running PostgreSQL instance
	code := m.Run()
	os.Exit(code)
}

func tearUp(t *testing.T) (*DB, func()) {
	t.Helper()

	// Create database using environment variables from .env file
	db := CreateTestDBWithEnv(t)
	return db, func() { db.Close() }
}
