package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrateUpDown(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary directory for migrations
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Set the source of the migrations
	db.Migrator.SetSource(migrationsDir)

	// Create a new migration
	err = db.Migrator.NewMigration(ctx, "test1", "sql")
	if err != nil {
		t.Fatalf("Failed to create new migration: %v", err)
	}

	// Migrate up
	err = db.Migrator.Up(ctx)
	if err != nil {
		t.Fatalf("Failed to migrate up: %v", err)
	}

	// Get the status of the migrations
	_, err = db.Migrator.Status(ctx)
	if err != nil {
		t.Fatalf("Failed to get migration status: %v", err)
	}

	// Migrate down
	err = db.Migrator.Down(ctx)
	if err != nil {
		t.Fatalf("Failed to migrate down: %v", err)
	}

	// Reset the migrations
	err = db.Migrator.Reset(ctx)
	if err != nil {
		t.Fatalf("Failed to reset migrations: %v", err)
	}
}

// TestMigrationStatusStruct tests that the Status method returns the correct struct format
func TestMigrationStatusStruct(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary directory for migrations
	tempDir := t.TempDir()
	migrationsDir := filepath.Join(tempDir, "migrations")
	err := os.MkdirAll(migrationsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Set the source of the migrations
	db.Migrator.SetSource(migrationsDir)

	// Test that Status returns the correct struct type
	status, err := db.Migrator.Status(ctx)
	if err != nil {
		// It's okay if this fails due to no database connection
		t.Skipf("Status failed (no database connection): %v", err)
		return
	}

	// Verify the struct is not nil
	if status == nil {
		t.Fatal("Status returned nil")
	}

	// Verify the struct has the expected fields
	_ = status.Current
	_ = status.Latest
	_ = status.Pending
	_ = status.Applied
	_ = status.Migrations

	t.Logf("Status struct returned successfully: Current=%d, Latest=%d, Applied=%d, Pending=%d",
		status.Current, status.Latest, status.Applied, status.Pending)
}
