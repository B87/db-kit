# db-kit

A comprehensive Go library for PostgreSQL database management with embedded database support, migrations, backup/restore functionality, and testing utilities.

## Features

- **Database Connection Management**: Robust connection pooling and retry logic
- **Embedded PostgreSQL**: Optional embedded database for development and testing
- **Migrations**: Goose-based migration system
- **Backup & Restore**: Interface-based backup and restore with PostgreSQL client tools
- **Testing Utilities**: Comprehensive test helpers and utilities
- **CLI Tools**: Command-line interface for database operations

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/b87/db-kit/database"
)

func main() {
    // Create database connection with default configuration
    db, err := database.NewDefault()
    if err != nil {
        log.Fatalf("Failed to create database: %v", err)
    }
    defer db.Close()

    // Use the database connection
    ctx := context.Background()
    if err := db.Ping(ctx); err != nil {
        log.Fatalf("Failed to ping database: %v", err)
    }

    log.Println("Database connection established successfully!")
}
```

### With Custom Configuration

```go
config := database.Config{
    Host:     "localhost",
    Port:     5432,
    User:     "myuser",
    Password: "mypassword",
    DBName:   "mydatabase",

    // Connection pool settings
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 5 * time.Minute,

    // SSL configuration
    SSLMode: "require",

    // Migration settings
    MigrationsDir: "./migrations",
    BackupsDir:    "./backups",
}

db, err := database.New(config)
if err != nil {
    log.Fatalf("Failed to create database: %v", err)
}
defer db.Close()
```

## Database Configuration

### Environment Variables

When using `NewDefault()`, the following environment variables are supported:

| Environment Variable | Default Value       | Description                           |
| -------------------- | ------------------- | ------------------------------------- |
| `POSTGRES_HOST`      | `localhost`         | Database host address                 |
| `POSTGRES_PORT`      | `5432`              | Database port number                  |
| `POSTGRES_USER`      | `postgres`          | Database user                         |
| `POSTGRES_PASSWORD`  | `postgres`          | Database password                     |
| `POSTGRES_DB`        | `postgres`          | Database name                         |
| `POSTGRES_SSL_MODE`  | `disable`           | SSL mode (disable, require, verify-ca, verify-full) |
| `POSTGRES_MAX_OPEN_CONNS` | `25`        | Maximum number of open connections   |
| `POSTGRES_MAX_IDLE_CONNS` | `5`         | Maximum number of idle connections   |
| `POSTGRES_CONN_MAX_LIFETIME` | `5m`    | Maximum lifetime of a connection     |
| `POSTGRES_CONN_MAX_IDLE_TIME` | `1m`   | Maximum idle time of a connection    |
| `POSTGRES_CONNECT_TIMEOUT` | `30s`      | Connection timeout                     |
| `POSTGRES_STATEMENT_TIMEOUT` | `30s`   | Statement execution timeout           |
| `POSTGRES_RETRY_ATTEMPTS` | `3`        | Number of retry attempts              |
| `POSTGRES_RETRY_DELAY` | `100ms`      | Initial delay between retries         |
| `POSTGRES_RETRY_MAX_DELAY` | `5s`    | Maximum delay between retries         |
| `POSTGRES_LOG_LEVEL` | `INFO`              | Logging level                         |
| `MIGRATIONS_DIR`     | `../tmp/migrations` | Directory containing Goose migrations |
| `BACKUPS_DIR`        | `../tmp/backups`    | Directory for database backups        |

### Configuration Struct

```go
type Config struct {
    // Basic connection settings
    Host     string
    Port     int
    User     string
    Password string
    DBName   string

    // SSL/TLS Configuration
    SSLMode     string // disable, require, verify-ca, verify-full
    SSLCert     string // path to client certificate
    SSLKey      string // path to client private key
    SSLRootCert string // path to root certificate

    // Connection Pool Configuration
    MaxOpenConns    int           // maximum number of open connections
    MaxIdleConns    int           // maximum number of idle connections
    ConnMaxLifetime time.Duration // maximum lifetime of a connection
    ConnMaxIdleTime time.Duration // maximum idle time of a connection

    // Connection Timeouts
    ConnectTimeout   time.Duration // connection timeout
    StatementTimeout time.Duration // statement execution timeout

    // Retry Configuration
    RetryAttempts int           // number of retry attempts for transient failures
    RetryDelay    time.Duration // initial delay between retries
    RetryMaxDelay time.Duration // maximum delay between retries

    // Logging Configuration
    Logger   *slog.Logger // structured logger instance
    LogLevel slog.Level   // minimum log level

    // Application-specific paths
    MigrationsDir string // goose migrations path
    BackupsDir    string // backup data path
}
```

## Backup and Restore

The library provides interface-based backup and restore functionality with PostgreSQL client tools.

### Backup Interface

```go
// Backuper defines the interface for database backup operations
type Backuper interface {
    Backup(ctx context.Context, config Config) error
    BackupToFile(ctx context.Context, config Config, filePath string) error
}
```

### Restore Interface

```go
// Restorer defines the interface for database restore operations
type Restorer interface {
    Restore(ctx context.Context, config Config, backupPath string) error
}
```

### Usage Examples

#### Basic Backup and Restore

```go
// Create database connection
db, err := database.NewDefault()
if err != nil {
    log.Fatalf("Failed to create database: %v", err)
}
defer db.Close()

ctx := context.Background()

// Create a backup with timestamped filename
if err := db.Backup(ctx); err != nil {
    log.Fatalf("Backup failed: %v", err)
}

// Create a backup to a specific file
if err := db.BackupToFile(ctx, "/path/to/backup.sql"); err != nil {
    log.Fatalf("Backup to file failed: %v", err)
}

// Restore from a backup file
if err := db.Restore(ctx, "/path/to/backup.sql"); err != nil {
    log.Fatalf("Restore failed: %v", err)
}
```

#### Custom Backup/Restore Implementations

```go
// Create custom implementations
backuper := database.NewPgDump()
restorer := database.NewPgRestore()

// Use implementations directly
config := db.Config()
if err := backuper.Backup(ctx, config); err != nil {
    log.Fatalf("Custom backup failed: %v", err)
}

if err := restorer.Restore(ctx, config, "/path/to/backup.sql"); err != nil {
    log.Fatalf("Custom restore failed: %v", err)
}
```

### Requirements

Backup and restore functionality requires PostgreSQL client tools:

- **pg_dump**: For creating database backups
- **pg_restore**: For restoring custom format backups
- **psql**: For restoring plain SQL backups

#### Installation

**Ubuntu/Debian:**
```bash
sudo apt-get install postgresql-client
```

**macOS:**
```bash
brew install postgresql
```

**Windows:**
Download from [PostgreSQL official website](https://www.postgresql.org/download/windows/)

## Migrations

The library uses Goose for database migrations.

### Migration Structure

```
migrations/
├── 001_create_users.sql
├── 002_create_posts.sql
└── 003_add_indexes.sql
```

### Migration Usage

```go
// Run migrations
if err := db.Migrator.Up(ctx); err != nil {
    log.Fatalf("Migration failed: %v", err)
}

// Check migration status
status, err := db.Migrator.Status(ctx)
if err != nil {
    log.Fatalf("Failed to get migration status: %v", err)
}

// Rollback migrations
if err := db.Migrator.Down(ctx); err != nil {
    log.Fatalf("Rollback failed: %v", err)
}
```

## Testing

The library provides comprehensive testing utilities.

### Test Database Setup

```go
func TestMyFunction(t *testing.T) {
    // Set up test database
    db, close := tearUp(t)
    defer close()

    // Your test code here
    ctx := context.Background()
    if err := db.Ping(ctx); err != nil {
        t.Fatalf("Failed to ping database: %v", err)
    }
}
```

### Environment Configuration for Tests

Create a `database/test.env` file for test-specific configuration:

```env
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=postgres
POSTGRES_SSL_MODE=disable
```

## CLI Usage

The library includes a command-line interface for common database operations.

### Available Commands

```bash
# Database status
./db-kit status

# Run migrations
./db-kit migrate up

# Create backup
./db-kit backup

# Restore from backup
./db-kit restore /path/to/backup.sql

# Health check
./db-kit health
```

## Error Handling

The library provides structured error handling with context and retry logic.

```go
// Check for specific error types
if database.IsRetriable(err) {
    // Handle retriable errors
}

// Get error code
code := database.GetErrorCode(err)
switch code {
case database.ErrCodeConnectionFailed:
    // Handle connection errors
case database.ErrCodeMigrationFailed:
    // Handle migration errors
case database.ErrCodeBackupFailed:
    // Handle backup errors
}
```

## Health Checks

```go
// Basic health check with retry
if err := db.HealthCheck(ctx); err != nil {
    log.Fatalf("Health check failed: %v", err)
}

// Health check without retry
if err := db.HealthCheckNoRetry(ctx); err != nil {
    log.Fatalf("Health check failed: %v", err)
}
```

## Connection Validation

```go
// Validate connection with retry
if err := db.ValidateConnection(ctx); err != nil {
    log.Fatalf("Connection validation failed: %v", err)
}

// Execute operations with automatic validation
if err := db.WithValidation(ctx, func() error {
    // Your database operations here
    return nil
}); err != nil {
    log.Fatalf("Operation failed: %v", err)
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
