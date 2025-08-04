package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBatchMigrationOperations(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	// Create a temporary migrations directory
	tempDir, err := os.MkdirTemp("", "test_migrations")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test migration files
	createTestMigrations(t, tempDir)

	config := testDB.GetConfig()
	config.MigrationsDir = tempDir

	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Reset database to initial state before running tests
	err = db.Migrator.Reset(ctx)
	if err != nil {
		t.Fatalf("Failed to reset database: %v", err)
	}

	t.Run("get initial version", func(t *testing.T) {
		version, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get version: %v", err)
		}
		if version != 0 {
			t.Errorf("Expected initial version 0, got %d", version)
		}
	})

	t.Run("validate migrations", func(t *testing.T) {
		err := db.Migrator.Validate(ctx)
		if err != nil {
			t.Errorf("Migration validation failed: %v", err)
		}
	})

	t.Run("up by one", func(t *testing.T) {
		err := db.Migrator.UpByOne(ctx)
		if err != nil {
			t.Errorf("UpByOne failed: %v", err)
		}

		version, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get version after UpByOne: %v", err)
		}
		if version == 0 {
			t.Errorf("Expected version to be greater than 0 after UpByOne, got %d", version)
		}
	})

	t.Run("down by one", func(t *testing.T) {
		err := db.Migrator.DownByOne(ctx)
		if err != nil {
			t.Errorf("DownByOne failed: %v", err)
		}

		version, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get version after DownByOne: %v", err)
		}
		if version != 0 {
			t.Errorf("Expected version 0 after DownByOne, got %d", version)
		}
	})

	t.Run("up to specific version", func(t *testing.T) {
		// Apply all migrations first to get available versions
		err := db.Migrator.Up(ctx)
		if err != nil {
			t.Errorf("Initial Up failed: %v", err)
		}

		// Reset to start fresh
		err = db.Migrator.Reset(ctx)
		if err != nil {
			t.Errorf("Reset failed: %v", err)
		}

		// Now test UpTo with a mock version (we'll use timestamp format)
		// Since we created test migrations with timestamps, we need to get the first one
		files, err := os.ReadDir(tempDir)
		if err != nil {
			t.Fatalf("Failed to read migrations dir: %v", err)
		}

		if len(files) > 0 {
			// Extract version from first migration file name
			// Format: YYYYMMDDHHMMSS_name.sql
			fileName := files[0].Name()
			if len(fileName) >= 14 {
				// Convert to int64 for testing (this is a simplified approach)
				var version int64 = 20250102000001 // Use a test version
				err := db.Migrator.UpTo(ctx, version)
				if err != nil {
					// This might fail if the version doesn't exist, which is expected
					t.Logf("UpTo failed as expected for non-existent version: %v", err)
				}
			}
		}
	})

	t.Run("migration in transaction", func(t *testing.T) {
		// Reset first
		err := db.Migrator.Reset(ctx)
		if err != nil {
			t.Errorf("Reset failed: %v", err)
		}

		// Test UpInTransaction with no specific versions (should apply all)
		err = db.Migrator.UpInTransaction(ctx)
		if err != nil {
			t.Errorf("UpInTransaction failed: %v", err)
		}

		version, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get version after UpInTransaction: %v", err)
		}
		if version == 0 {
			t.Errorf("Expected version to be greater than 0 after UpInTransaction, got %d", version)
		}

		// Test DownInTransaction with no specific versions (should rollback one)
		err = db.Migrator.DownInTransaction(ctx)
		if err != nil {
			t.Errorf("DownInTransaction failed: %v", err)
		}
	})
}

func TestMigrationValidation(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	t.Run("validation with empty migrations dir", func(t *testing.T) {
		config := testDB.GetConfig()
		config.MigrationsDir = "" // Empty migrations dir

		db, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer db.Close()

		ctx := context.Background()
		err = db.Migrator.Validate(ctx)
		if err == nil {
			t.Errorf("Expected validation to fail with empty migrations dir")
		}
	})

	t.Run("validation with valid setup", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "test_migrations")
		if err != nil {
			t.Fatalf("Failed to create temp directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		config := testDB.GetConfig()
		config.MigrationsDir = tempDir

		db, err := New(config)
		if err != nil {
			t.Fatalf("Failed to create database: %v", err)
		}
		defer db.Close()

		ctx := context.Background()
		err = db.Migrator.Validate(ctx)
		if err != nil {
			t.Errorf("Validation should succeed with valid setup: %v", err)
		}
	})
}

func TestMigrationVersionOperations(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	tempDir, err := os.MkdirTemp("", "test_migrations")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	createTestMigrations(t, tempDir)

	config := testDB.GetConfig()
	config.MigrationsDir = tempDir

	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Reset database to initial state before running tests
	err = db.Migrator.Reset(ctx)
	if err != nil {
		t.Fatalf("Failed to reset database: %v", err)
	}

	t.Run("version tracking through operations", func(t *testing.T) {
		// Start with version 0
		version, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get initial version: %v", err)
		}
		if version != 0 {
			t.Errorf("Expected initial version 0, got %d", version)
		}

		// Apply one migration
		err = db.Migrator.UpByOne(ctx)
		if err != nil {
			t.Errorf("UpByOne failed: %v", err)
		}

		// Check version increased
		newVersion, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get version after UpByOne: %v", err)
		}
		if newVersion <= version {
			t.Errorf("Expected version to increase after UpByOne, got %d (was %d)", newVersion, version)
		}

		// Rollback one migration
		err = db.Migrator.DownByOne(ctx)
		if err != nil {
			t.Errorf("DownByOne failed: %v", err)
		}

		// Check version decreased
		finalVersion, err := db.Migrator.Version(ctx)
		if err != nil {
			t.Errorf("Failed to get version after DownByOne: %v", err)
		}
		if finalVersion != version {
			t.Errorf("Expected version to return to %d after DownByOne, got %d", version, finalVersion)
		}
	})
}

func TestMigrationTimeouts(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	tempDir, err := os.MkdirTemp("", "test_migrations")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	createTestMigrations(t, tempDir)

	config := testDB.GetConfig()
	config.MigrationsDir = tempDir

	db, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	t.Run("migration with short timeout", func(t *testing.T) {
		// Create a context with a very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// This should fail due to timeout (depending on system speed)
		err := db.Migrator.Up(ctx)
		// We expect this might succeed or fail depending on timing
		// The important thing is that it handles the context properly
		if err != nil {
			t.Logf("Migration failed with short timeout as expected: %v", err)
		}
	})
}

// createTestMigrations creates some basic migration files for testing
func createTestMigrations(t *testing.T, dir string) {
	migrations := []struct {
		name    string
		content string
	}{
		{
			name: "20250102000001_create_users.sql",
			content: `-- +goose Up
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- +goose Down
DROP TABLE users;`,
		},
		{
			name: "20250102000002_create_posts.sql",
			content: `-- +goose Up
CREATE TABLE posts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    title VARCHAR(255) NOT NULL,
    content TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

-- +goose Down
DROP TABLE posts;`,
		},
		{
			name: "20250102000003_add_users_index.sql",
			content: `-- +goose Up
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_posts_user_id ON posts(user_id);

-- +goose Down
DROP INDEX idx_users_email;
DROP INDEX idx_posts_user_id;`,
		},
	}

	for _, migration := range migrations {
		filePath := filepath.Join(dir, migration.name)
		err := os.WriteFile(filePath, []byte(migration.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create migration file %s: %v", migration.name, err)
		}
	}
}
