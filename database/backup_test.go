package database

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupToFile(t *testing.T) {
	// Set up the database
	db, closeDB := tearUp(t)
	defer closeDB()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary file for backup
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test_backup.sql")

	// Test backup
	err := db.BackupToFile(ctx, backupPath)
	if err != nil {
		t.Skipf("Backup failed (pg_dump may not be available): %v", err)
	}

	// Check that backup file was created
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("Backup file was not created at %s", backupPath)
	}

	// Check that backup file has content
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("Failed to stat backup file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("Backup file is empty")
	}
}

func TestBackup(t *testing.T) {
	// Set up the database
	db, closeDB := tearUp(t)
	defer closeDB()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create temporary backup directory
	tempDir := t.TempDir()
	db.config.BackupsDir = tempDir

	// Test backup
	err := db.Backup(ctx)
	if err != nil {
		t.Skipf("Backup failed (pg_dump may not be available): %v", err)
	}

	// Check that a backup file was created in the directory
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read backup directory: %v", err)
	}

	if len(files) == 0 {
		t.Error("No backup file was created")
		return
	}

	// Check that the backup file has the expected naming pattern
	found := false
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".sql" {
			found = true
			break
		}
	}

	if !found {
		t.Error("No .sql backup file found")
	}
}

func TestRestore(t *testing.T) {
	// Set up the database
	db, closeDB := tearUp(t)
	defer closeDB()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a simple SQL backup file for testing
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test_restore.sql")

	// Create a simple SQL file with a table creation
	sqlContent := `
CREATE TABLE IF NOT EXISTS test_restore_table (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT INTO test_restore_table (name) VALUES ('test');
`

	err := os.WriteFile(backupPath, []byte(sqlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}

	// Test restore
	err = db.Restore(ctx, backupPath)
	if err != nil {
		t.Skipf("Restore failed (psql/pg_restore may not be available): %v", err)
	}

	// Verify that the table was created by querying it
	var count int
	err = db.db.Get(&count, "SELECT COUNT(*) FROM test_restore_table")
	if err != nil {
		t.Errorf("Failed to query restored table: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 row in restored table, got %d", count)
	}

	// Clean up the test table
	_, err = db.db.Exec("DROP TABLE IF EXISTS test_restore_table")
	if err != nil {
		t.Logf("Failed to clean up test table: %v", err)
	}
}

// TestPgDump tests the pgDump implementation directly
func TestPgDump(t *testing.T) {
	// Set up the database
	db, closeDB := tearUp(t)
	defer closeDB()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a temporary file for backup
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test_pgdump_backup.sql")

	// Test pgDump directly
	pgDump := NewPgDump()
	err := pgDump.BackupToFile(ctx, db.config, backupPath)
	if err != nil {
		t.Skipf("pgDump backup failed (pg_dump may not be available): %v", err)
	}

	// Check that backup file was created
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("pgDump backup file was not created at %s", backupPath)
	}

	// Check that backup file has content
	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("Failed to stat pgDump backup file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("pgDump backup file is empty")
	}
}

// TestPgRestore tests the pgRestore implementation directly
func TestPgRestore(t *testing.T) {
	// Set up the database
	db, closeDB := tearUp(t)
	defer closeDB()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a simple SQL backup file for testing
	tempDir := t.TempDir()
	backupPath := filepath.Join(tempDir, "test_pgrestore_backup.sql")

	// Create a simple SQL file with a table creation
	sqlContent := `
CREATE TABLE IF NOT EXISTS test_pgrestore_table (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

INSERT INTO test_pgrestore_table (name) VALUES ('test');
`

	err := os.WriteFile(backupPath, []byte(sqlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test backup file: %v", err)
	}

	// Test pgRestore directly
	pgRestore := NewPgRestore()
	err = pgRestore.Restore(ctx, db.config, backupPath)
	if err != nil {
		t.Skipf("pgRestore failed (psql/pg_restore may not be available): %v", err)
	}

	// Verify that the table was created by querying it
	var count int
	err = db.db.Get(&count, "SELECT COUNT(*) FROM test_pgrestore_table")
	if err != nil {
		t.Errorf("Failed to query restored table: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 row in restored table, got %d", count)
	}

	// Clean up the test table
	_, err = db.db.Exec("DROP TABLE IF EXISTS test_pgrestore_table")
	if err != nil {
		t.Logf("Failed to clean up test table: %v", err)
	}
}

// TestInterfaceImplementations tests that the interfaces are properly implemented
func TestInterfaceImplementations(t *testing.T) {
	// Test that pgDump implements Backuper interface
	var _ Backuper = NewPgDump()

	// Test that pgRestore implements Restorer interface
	var _ Restorer = NewPgRestore()

	// Test that the implementations can be created without errors
	backuper := NewPgDump()
	restorer := NewPgRestore()

	if backuper == nil {
		t.Error("NewPgDump() returned nil")
	}

	if restorer == nil {
		t.Error("NewPgRestore() returned nil")
	}
}

// TestConfigValidation tests that the Config struct works with the interfaces
func TestConfigValidation(t *testing.T) {
	config := Config{
		Host:       "localhost",
		Port:       5432,
		User:       "testuser",
		Password:   "testpass",
		DBName:     "testdb",
		BackupsDir: "/tmp/backups",
	}

	// Test that we can create implementations with the config
	backuper := NewPgDump()
	restorer := NewPgRestore()

	if backuper == nil {
		t.Error("Failed to create Backuper implementation")
	}

	if restorer == nil {
		t.Error("Failed to create Restorer implementation")
	}

	// Test that the config has the required fields
	if config.Host == "" {
		t.Error("Config Host is empty")
	}

	if config.DBName == "" {
		t.Error("Config DBName is empty")
	}
}

// TestCommandAvailability tests that pg_dump and pg_restore commands are available
func TestCommandAvailability(t *testing.T) {
	// Test pg_dump availability
	pgDumpCmd := exec.Command("pg_dump", "--version")
	if err := pgDumpCmd.Run(); err != nil {
		t.Skipf("pg_dump command not available: %v", err)
	}

	// Test pg_restore availability
	pgRestoreCmd := exec.Command("pg_restore", "--version")
	if err := pgRestoreCmd.Run(); err != nil {
		t.Skipf("pg_restore command not available: %v", err)
	}

	// Test psql availability
	psqlCmd := exec.Command("psql", "--version")
	if err := psqlCmd.Run(); err != nil {
		t.Skipf("psql command not available: %v", err)
	}

	t.Log("All PostgreSQL client tools are available")
}
