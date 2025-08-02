package database

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"
)

// MigrationStatus represents the status of a single migration
type MigrationStatus struct {
	Version     int64     `json:"version"`
	AppliedAt   time.Time `json:"applied_at"`
	Source      string    `json:"source"`
	IsApplied   bool      `json:"is_applied"`
	Description string    `json:"description"`
}

// MigrationStatusResult represents the complete migration status
type MigrationStatusResult struct {
	Migrations []MigrationStatus `json:"migrations"`
	Current    int64             `json:"current_version"`
	Latest     int64             `json:"latest_version"`
	Pending    int               `json:"pending_count"`
	Applied    int               `json:"applied_count"`
}

// Migrator is an interface that interacts with the database migrations
type Migrator interface {
	// Apply migrations to the database
	Up(ctx context.Context) error
	// Rollback migrations to the database
	Down(ctx context.Context) error
	// Reset the database to the initial state
	Reset(ctx context.Context) error
	// Get the status of the migrations
	Status(ctx context.Context) (*MigrationStatusResult, error)
	// Create a new migration file
	NewMigration(ctx context.Context, name, migrationType string) error
	// Get the source of the migrations
	Source() string
	// Set the source of the migrations
	SetSource(source string)

	// Batch migration operations
	// Apply migrations up to a specific version
	UpTo(ctx context.Context, version int64) error
	// Apply migrations by a specific count
	UpByOne(ctx context.Context) error
	// Rollback migrations by a specific count
	DownTo(ctx context.Context, version int64) error
	// Rollback one migration
	DownByOne(ctx context.Context) error
	// Get migration version information
	Version(ctx context.Context) (int64, error)
	// Apply multiple migrations in a transaction
	UpInTransaction(ctx context.Context, versions ...int64) error
	// Rollback multiple migrations in a transaction
	DownInTransaction(ctx context.Context, versions ...int64) error
	// Validate migrations before applying
	Validate(ctx context.Context) error
}

// GooseMigrator is a concrete implementation of the Migrator interface
type GooseMigrator struct {
	db            *sqlx.DB
	migrationsDir string
}

// NewGooseMigrator creates a new GooseMigrator
func NewGooseMigrator(db *sqlx.DB, migrationsDir string) *GooseMigrator {
	return &GooseMigrator{db: db, migrationsDir: migrationsDir}
}

// Up applies the migrations to the database
func (migrator *GooseMigrator) Up(ctx context.Context) error {
	return goose.UpContext(ctx, migrator.db.DB, migrator.migrationsDir)
}

// Down rolls back the migrations to the database
func (migrator *GooseMigrator) Down(ctx context.Context) error {
	return goose.DownContext(ctx, migrator.db.DB, migrator.migrationsDir)
}

// Reset resets the database to the initial state
func (migrator *GooseMigrator) Reset(ctx context.Context) error {
	return goose.ResetContext(ctx, migrator.db.DB, migrator.migrationsDir)
}

// Status gets the status of the migrations
func (migrator *GooseMigrator) Status(ctx context.Context) (*MigrationStatusResult, error) {
	// Scan migrations directory for migration files
	files, err := os.ReadDir(migrator.migrationsDir)
	if err != nil {
		return nil, NewMigrationError("failed to read migrations directory", err).
			WithOperation("get_status")
	}

	// Get current database version
	currentVersion, err := goose.GetDBVersionContext(ctx, migrator.db.DB)
	if err != nil {
		return nil, NewMigrationError("failed to get current version", err).
			WithOperation("get_status")
	}

	// Build migration status list
	var migrationStatuses []MigrationStatus
	appliedCount := 0
	pendingCount := 0
	latestVersion := int64(0)

	// Process each migration file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		if !strings.HasSuffix(filename, ".sql") && !strings.HasSuffix(filename, ".go") {
			continue
		}

		// Extract version from filename (e.g., "001_create_users.sql" -> 1)
		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			continue
		}

		versionStr := parts[0]
		version, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			continue // Skip files that don't start with a number
		}

		// Check if this migration is applied by querying the database
		isApplied := false
		var appliedAt time.Time

		// Query the goose_db_version table to check if migration is applied
		var dbVersion int64
		var tstamp time.Time
		err = migrator.db.QueryRowContext(ctx,
			"SELECT version_id, tstamp FROM goose_db_version WHERE version_id = ?",
			version).Scan(&dbVersion, &tstamp)

		if err == nil {
			isApplied = true
			appliedAt = tstamp
			appliedCount++
		} else {
			pendingCount++
		}

		status := MigrationStatus{
			Version:     version,
			AppliedAt:   appliedAt,
			Source:      filename,
			IsApplied:   isApplied,
			Description: filename,
		}
		migrationStatuses = append(migrationStatuses, status)

		// Track latest version
		if version > latestVersion {
			latestVersion = version
		}
	}

	// Sort migrations by version
	sort.Slice(migrationStatuses, func(i, j int) bool {
		return migrationStatuses[i].Version < migrationStatuses[j].Version
	})

	return &MigrationStatusResult{
		Migrations: migrationStatuses,
		Current:    currentVersion,
		Latest:     latestVersion,
		Pending:    pendingCount,
		Applied:    appliedCount,
	}, nil
}

// NewMigration creates a new migration file
func (migrator *GooseMigrator) NewMigration(ctx context.Context, name, migrationType string) error {
	// goose.Create doesn't have a context version, but it's a quick file operation
	return goose.Create(migrator.db.DB, migrator.migrationsDir, name, migrationType)
}

// Source gets the source of the migrations
func (migrator *GooseMigrator) Source() string {
	return migrator.migrationsDir
}

// SetSource sets the source of the migrations
func (migrator *GooseMigrator) SetSource(source string) {
	migrator.migrationsDir = source
}

// UpTo applies migrations up to a specific version
func (migrator *GooseMigrator) UpTo(ctx context.Context, version int64) error {
	err := goose.UpToContext(ctx, migrator.db.DB, migrator.migrationsDir, version)
	if err != nil {
		return NewMigrationError(fmt.Sprintf("failed to migrate up to version %d", version), err).
			WithContext("target_version", version).
			WithOperation("migrate_up_to")
	}
	return nil
}

// UpByOne applies one migration
func (migrator *GooseMigrator) UpByOne(ctx context.Context) error {
	err := goose.UpByOneContext(ctx, migrator.db.DB, migrator.migrationsDir)
	if err != nil {
		return NewMigrationError("failed to migrate up by one", err).
			WithOperation("migrate_up_by_one")
	}
	return nil
}

// DownTo rolls back migrations to a specific version
func (migrator *GooseMigrator) DownTo(ctx context.Context, version int64) error {
	err := goose.DownToContext(ctx, migrator.db.DB, migrator.migrationsDir, version)
	if err != nil {
		return NewMigrationError(fmt.Sprintf("failed to migrate down to version %d", version), err).
			WithContext("target_version", version).
			WithOperation("migrate_down_to")
	}
	return nil
}

// DownByOne rolls back one migration
func (migrator *GooseMigrator) DownByOne(ctx context.Context) error {
	err := goose.DownContext(ctx, migrator.db.DB, migrator.migrationsDir)
	if err != nil {
		return NewMigrationError("failed to migrate down by one", err).
			WithOperation("migrate_down_by_one")
	}
	return nil
}

// Version gets the current migration version
func (migrator *GooseMigrator) Version(ctx context.Context) (int64, error) {
	version, err := goose.GetDBVersionContext(ctx, migrator.db.DB)
	if err != nil {
		return 0, NewMigrationError("failed to get database version", err).
			WithOperation("get_version")
	}
	return version, nil
}

// UpInTransaction applies multiple migrations with validation and error recovery
// Note: Goose doesn't support transactions for migrations directly, so this provides
// validation and rollback capabilities instead
func (migrator *GooseMigrator) UpInTransaction(ctx context.Context, versions ...int64) error {
	if len(versions) == 0 {
		// If no specific versions, apply all pending migrations
		return migrator.Up(ctx)
	}

	// Get current version for potential rollback
	currentVersion, err := migrator.Version(ctx)
	if err != nil {
		return NewMigrationError("failed to get current version before batch migration", err).
			WithOperation("migrate_up_transaction")
	}

	// Apply each migration version with rollback on failure
	for _, version := range versions {
		err = migrator.UpTo(ctx, version)
		if err != nil {
			// Rollback to original version on any failure
			if rollbackErr := migrator.DownTo(ctx, currentVersion); rollbackErr != nil {
				return NewMigrationError(fmt.Sprintf("failed to migrate to version %d and rollback failed", version), err).
					WithContext("target_version", version).
					WithContext("rollback_error", rollbackErr.Error()).
					WithOperation("migrate_up_transaction")
			}
			return NewMigrationError(fmt.Sprintf("failed to migrate to version %d in batch operation", version), err).
				WithContext("target_version", version).
				WithOperation("migrate_up_transaction")
		}
	}

	return nil
}

// DownInTransaction rolls back multiple migrations with validation and error recovery
func (migrator *GooseMigrator) DownInTransaction(ctx context.Context, versions ...int64) error {
	if len(versions) == 0 {
		// If no specific versions, rollback one migration
		return migrator.DownByOne(ctx)
	}

	// Get current version for potential recovery
	currentVersion, err := migrator.Version(ctx)
	if err != nil {
		return NewMigrationError("failed to get current version before batch rollback", err).
			WithOperation("migrate_down_transaction")
	}

	// Rollback to each version (in reverse order to be safe)
	for i := len(versions) - 1; i >= 0; i-- {
		version := versions[i]
		err = migrator.DownTo(ctx, version)
		if err != nil {
			// Attempt to restore to original version on failure
			if restoreErr := migrator.UpTo(ctx, currentVersion); restoreErr != nil {
				return NewMigrationError(fmt.Sprintf("failed to rollback to version %d and restore failed", version), err).
					WithContext("target_version", version).
					WithContext("restore_error", restoreErr.Error()).
					WithOperation("migrate_down_transaction")
			}
			return NewMigrationError(fmt.Sprintf("failed to rollback to version %d in batch operation", version), err).
				WithContext("target_version", version).
				WithOperation("migrate_down_transaction")
		}
	}

	return nil
}

// Validate validates migrations before applying them
func (migrator *GooseMigrator) Validate(ctx context.Context) error {
	// Check if migrations directory exists
	if migrator.migrationsDir == "" {
		return NewValidationError("migrations directory not set", nil).
			WithOperation("validate_migrations")
	}

	// Try to set the dialect (this will validate the database connection)
	if err := goose.SetDialect("postgres"); err != nil {
		return NewValidationError("failed to set database dialect", err).
			WithOperation("validate_migrations")
	}

	// Check current version to ensure the database is accessible
	_, err := migrator.Version(ctx)
	if err != nil {
		return WrapError(err, ErrCodeValidation, "validate_migrations", "failed to validate database connection")
	}

	return nil
}
