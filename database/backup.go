// Package database provides database connection, migration, backup, and restore functionality.
package database

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// Backuper defines the interface for database backup operations
type Backuper interface {
	Backup(ctx context.Context, config Config) error
	BackupToFile(ctx context.Context, config Config, filePath string) error
}

// Restorer defines the interface for database restore operations
type Restorer interface {
	Restore(ctx context.Context, config Config, backupPath string) error
}

// pgDump implements the Backuper interface using pg_dump
type pgDump struct{}

// NewPgDump creates a new pgDump instance
func NewPgDump() Backuper {
	return &pgDump{}
}

// Backup creates a database backup using pg_dump with timestamped filename
func (p *pgDump) Backup(ctx context.Context, config Config) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("backup_%s_%s.sql", config.DBName, timestamp)
	backupPath := filepath.Join(config.BackupsDir, filename)

	return p.BackupToFile(ctx, config, backupPath)
}

// BackupToFile creates a database backup to a specific file path
func (p *pgDump) BackupToFile(ctx context.Context, config Config, filePath string) error {
	cmd := exec.CommandContext(ctx, "pg_dump",
		"--host", config.Host,
		"--port", fmt.Sprintf("%d", config.Port),
		"--username", config.User,
		"--dbname", config.DBName,
		"--file", filePath,
		"--verbose",
		"--no-password",
	)

	// Set PGPASSWORD environment variable for authentication
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", config.Password))

	if err := cmd.Run(); err != nil {
		return NewBackupError("pg_dump command failed", err).
			WithContext("backup_path", filePath).
			WithContext("database", config.DBName).
			WithOperation("backup")
	}
	return nil
}

// pgRestore implements the Restorer interface using pg_restore and psql
type pgRestore struct{}

// NewPgRestore creates a new pgRestore instance
func NewPgRestore() Restorer {
	return &pgRestore{}
}

// Restore restores a database from a backup file using pg_restore or psql
func (p *pgRestore) Restore(ctx context.Context, config Config, backupPath string) error {
	// First try with pg_restore (for custom format dumps)
	cmd := exec.CommandContext(ctx, "pg_restore",
		"--host", config.Host,
		"--port", fmt.Sprintf("%d", config.Port),
		"--username", config.User,
		"--dbname", config.DBName,
		"--verbose",
		"--no-password",
		"--clean",
		"--if-exists",
		backupPath,
	)

	// Set PGPASSWORD environment variable for authentication
	cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", config.Password))

	err := cmd.Run()
	if err != nil {
		// If pg_restore fails, try with psql (for plain SQL dumps)
		cmd = exec.CommandContext(ctx, "psql",
			"--host", config.Host,
			"--port", fmt.Sprintf("%d", config.Port),
			"--username", config.User,
			"--dbname", config.DBName,
			"--file", backupPath,
		)

		cmd.Env = append(cmd.Env, fmt.Sprintf("PGPASSWORD=%s", config.Password))
		if err := cmd.Run(); err != nil {
			return NewRestoreError("both pg_restore and psql commands failed", err).
				WithContext("backup_path", backupPath).
				WithContext("database", config.DBName).
				WithOperation("restore")
		}
	}

	return nil
}
