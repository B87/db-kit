package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// DB represents a database connection
type DB struct {
	// Apply migrations to the database
	Migrator Migrator

	// Backup and restore operations
	Backuper Backuper
	Restorer Restorer

	db     *sqlx.DB
	config Config
	logger *slog.Logger
}

// Config represents the configuration for a database connection
type Config struct {
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

// ConnectionString returns a connection string for the database
func (c Config) ConnectionString() string {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName)

	// Add SSL configuration
	if c.SSLMode != "" {
		connStr += fmt.Sprintf(" sslmode=%s", c.SSLMode)
	} else {
		connStr += " sslmode=disable"
	}

	if c.SSLCert != "" {
		connStr += fmt.Sprintf(" sslcert=%s", c.SSLCert)
	}

	if c.SSLKey != "" {
		connStr += fmt.Sprintf(" sslkey=%s", c.SSLKey)
	}

	if c.SSLRootCert != "" {
		connStr += fmt.Sprintf(" sslrootcert=%s", c.SSLRootCert)
	}

	// Add timeout configuration
	if c.ConnectTimeout > 0 {
		connStr += fmt.Sprintf(" connect_timeout=%d", int(c.ConnectTimeout.Seconds()))
	}

	if c.StatementTimeout > 0 {
		connStr += fmt.Sprintf(" statement_timeout=%d", int(c.StatementTimeout.Milliseconds()))
	}

	return connStr
}

// New creates a new database connection with the given configuration
func New(config Config) (*DB, error) {
	sqlxConn, err := sqlx.Connect("postgres", config.ConnectionString())
	if err != nil {
		return nil, NewConnectionError("failed to establish database connection", err).
			WithContext("host", config.Host).
			WithContext("port", config.Port).
			WithContext("database", config.DBName)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		sqlxConn.SetMaxOpenConns(config.MaxOpenConns)
	}

	if config.MaxIdleConns > 0 {
		sqlxConn.SetMaxIdleConns(config.MaxIdleConns)
	}

	if config.ConnMaxLifetime > 0 {
		sqlxConn.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	if config.ConnMaxIdleTime > 0 {
		sqlxConn.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}

	// Set up logger
	logger := config.Logger
	if logger == nil {
		// Create default logger if none provided
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: config.LogLevel,
		}))
	}

	db := &DB{
		db:       sqlxConn,
		config:   config,
		logger:   logger,
		Migrator: NewGooseMigrator(sqlxConn, config.MigrationsDir),
		Backuper: NewPgDump(),
		Restorer: NewPgRestore(),
	}

	logger.Debug("database connection established",
		slog.String("host", config.Host),
		slog.Int("port", config.Port),
		slog.String("database", config.DBName),
		slog.Int("max_open_conns", config.MaxOpenConns),
		slog.Int("max_idle_conns", config.MaxIdleConns),
	)

	return db, nil
}

// NewDefault creates a new database connection with default configuration
func NewDefault() (*DB, error) {
	port, err := strconv.Atoi(envOrDefault("POSTGRES_PORT", "5432"))
	if err != nil {
		return nil, NewConfigError("invalid POSTGRES_PORT value", err).
			WithContext("port_value", envOrDefault("POSTGRES_PORT", "5432"))
	}

	// Parse connection pool settings
	maxOpenConns, _ := strconv.Atoi(envOrDefault("POSTGRES_MAX_OPEN_CONNS", "25"))
	maxIdleConns, _ := strconv.Atoi(envOrDefault("POSTGRES_MAX_IDLE_CONNS", "5"))

	connMaxLifetime, _ := time.ParseDuration(envOrDefault("POSTGRES_CONN_MAX_LIFETIME", "5m"))
	connMaxIdleTime, _ := time.ParseDuration(envOrDefault("POSTGRES_CONN_MAX_IDLE_TIME", "1m"))
	connectTimeout, _ := time.ParseDuration(envOrDefault("POSTGRES_CONNECT_TIMEOUT", "30s"))
	statementTimeout, _ := time.ParseDuration(envOrDefault("POSTGRES_STATEMENT_TIMEOUT", "30s"))

	// Parse retry settings
	retryAttempts, _ := strconv.Atoi(envOrDefault("POSTGRES_RETRY_ATTEMPTS", "3"))
	retryDelay, _ := time.ParseDuration(envOrDefault("POSTGRES_RETRY_DELAY", "100ms"))
	retryMaxDelay, _ := time.ParseDuration(envOrDefault("POSTGRES_RETRY_MAX_DELAY", "5s"))

	// Parse log level
	logLevel := parseLogLevel(envOrDefault("POSTGRES_LOG_LEVEL", "INFO"))

	config := Config{
		Host:     envOrDefault("POSTGRES_HOST", "localhost"),
		Port:     port,
		User:     envOrDefault("POSTGRES_USER", "postgres"),
		Password: envOrDefault("POSTGRES_PASSWORD", "postgres"),
		DBName:   envOrDefault("POSTGRES_DB", "postgres"),

		// SSL Configuration
		SSLMode:     envOrDefault("POSTGRES_SSL_MODE", "disable"),
		SSLCert:     envOrDefault("POSTGRES_SSL_CERT", ""),
		SSLKey:      envOrDefault("POSTGRES_SSL_KEY", ""),
		SSLRootCert: envOrDefault("POSTGRES_SSL_ROOT_CERT", ""),

		// Connection Pool Configuration
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,

		// Connection Timeouts
		ConnectTimeout:   connectTimeout,
		StatementTimeout: statementTimeout,

		// Retry Configuration
		RetryAttempts: retryAttempts,
		RetryDelay:    retryDelay,
		RetryMaxDelay: retryMaxDelay,

		// Logging Configuration
		LogLevel: logLevel,

		// Application paths
		MigrationsDir: envOrDefault("MIGRATIONS_DIR", "../tmp/migrations"),
		BackupsDir:    envOrDefault("BACKUPS_DIR", "../tmp"),
	}
	return New(config)
}

// Close should be called when the application is shutting down.
func (d *DB) Close() error {
	return d.db.Close()
}

// DB returns the underlying *sqlx.DB instance
func (d *DB) DB() *sqlx.DB {
	return d.db
}

// Introspection returns a new introspection service for this database
func (d *DB) Introspection() *IntrospectionService {
	return NewIntrospectionService(d)
}

// Ping checks if the database connection is alive with retry logic
func (d *DB) Ping(ctx context.Context) error {
	d.logger.Debug("pinging database")
	err := d.withRetry(ctx, func() error {
		return d.db.PingContext(ctx)
	})
	if err != nil {
		d.logger.Error("database ping failed", slog.Any("error", err))
		return WrapError(err, ErrCodeConnectionFailed, "ping", "database ping failed")
	}
	d.logger.Debug("database ping successful")
	return nil
}

// PingNoRetry checks if the database connection is alive without retry logic
func (d *DB) PingNoRetry(ctx context.Context) error {
	return d.db.PingContext(ctx)
}

// HealthCheck performs a comprehensive health check with retry logic
func (d *DB) HealthCheck(ctx context.Context) error {
	err := d.withRetry(ctx, func() error {
		// Check connection
		if err := d.db.PingContext(ctx); err != nil {
			return WrapError(err, ErrCodeConnectionFailed, "health_check_ping", "health check ping failed")
		}

		// Check if we can execute a simple query
		var result int
		if err := d.db.GetContext(ctx, &result, "SELECT 1"); err != nil {
			return WrapError(err, ErrCodeQueryFailed, "health_check_query", "health check query failed")
		}

		return nil
	})

	if err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "health_check", "database health check failed")
	}
	return nil
}

// HealthCheckNoRetry performs a comprehensive health check without retry logic
func (d *DB) HealthCheckNoRetry(ctx context.Context) error {
	// Check connection
	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if we can execute a simple query
	var result int
	if err := d.db.GetContext(ctx, &result, "SELECT 1"); err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	return nil
}

// ValidateConnection checks if the connection is healthy and reconnects if needed
func (d *DB) ValidateConnection(ctx context.Context) error {
	d.logger.Debug("validating database connection")

	// First try a quick ping without retry
	if err := d.PingNoRetry(ctx); err != nil {
		d.logger.Warn("connection validation failed, attempting reconnection", slog.Any("error", err))

		// If ping fails, try to reconnect
		if err := d.reconnect(); err != nil {
			d.logger.Error("failed to reconnect to database", slog.Any("error", err))
			return NewConnectionError("failed to reconnect to database", err).
				WithOperation("validate_connection")
		}

		d.logger.Info("database reconnection successful")

		// Try ping again after reconnecting
		if err := d.PingNoRetry(ctx); err != nil {
			d.logger.Error("database still unreachable after reconnection", slog.Any("error", err))
			return NewConnectionError("database still unreachable after reconnection", err).
				WithOperation("validate_connection")
		}
	}

	d.logger.Debug("database connection validation successful")
	return nil
}

// reconnect attempts to re-establish the database connection
func (d *DB) reconnect() error {
	// Close existing connection
	if d.db != nil {
		d.db.Close()
	}

	// Create new connection
	sqlxConn, err := sqlx.Connect("postgres", d.config.ConnectionString())
	if err != nil {
		return NewConnectionError("failed to re-establish database connection", err).
			WithContext("host", d.config.Host).
			WithContext("port", d.config.Port).
			WithContext("database", d.config.DBName)
	}

	// Configure connection pool
	if d.config.MaxOpenConns > 0 {
		sqlxConn.SetMaxOpenConns(d.config.MaxOpenConns)
	}

	if d.config.MaxIdleConns > 0 {
		sqlxConn.SetMaxIdleConns(d.config.MaxIdleConns)
	}

	if d.config.ConnMaxLifetime > 0 {
		sqlxConn.SetConnMaxLifetime(d.config.ConnMaxLifetime)
	}

	if d.config.ConnMaxIdleTime > 0 {
		sqlxConn.SetConnMaxIdleTime(d.config.ConnMaxIdleTime)
	}

	// Update the connection
	d.db = sqlxConn
	d.Migrator = NewGooseMigrator(sqlxConn, d.config.MigrationsDir)

	d.logger.Info("database connection re-established")
	return nil
}

// WithValidation wraps an operation with connection validation
func (d *DB) WithValidation(ctx context.Context, operation func() error) error {
	// Validate connection before operation
	if err := d.ValidateConnection(ctx); err != nil {
		return WrapError(err, ErrCodeConnectionFailed, "with_validation", "connection validation failed")
	}

	// Execute operation with retry logic
	err := d.withRetry(ctx, operation)
	if err != nil {
		return WrapError(err, ErrCodeOperationTimeout, "with_validation", "operation failed after validation")
	}
	return nil
}

func envOrDefault(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

// isRetriableError checks if an error is retriable (transient failure)
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// Check for syscall errors
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Err == syscall.ECONNREFUSED ||
			opErr.Err == syscall.ECONNRESET ||
			opErr.Err == syscall.ETIMEDOUT {
			return true
		}
	}

	// Check for PostgreSQL specific errors
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		// PostgreSQL error codes for retriable errors
		switch pqErr.Code {
		case "53000", // insufficient_resources
			"53100", // disk_full
			"53200", // out_of_memory
			"53300", // too_many_connections
			"08000", // connection_exception
			"08003", // connection_does_not_exist
			"08006", // connection_failure
			"08001", // sqlclient_unable_to_establish_sqlconnection
			"08004": // sqlserver_rejected_establishment_of_sqlconnection
			return true
		}
	}

	// Check for driver errors
	if errors.Is(err, driver.ErrBadConn) {
		return true
	}

	// Check for context timeout (might be worth retrying)
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for common transient error messages
	errMsg := strings.ToLower(err.Error())
	transientMessages := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"network is unreachable",
		"temporary failure",
		"server is not available",
		"database is starting up",
	}

	for _, msg := range transientMessages {
		if strings.Contains(errMsg, msg) {
			return true
		}
	}

	return false
}

// RetryConfig holds retry configuration
type RetryConfig struct {
	Attempts int
	Delay    time.Duration
	MaxDelay time.Duration
}

// withRetry executes a function with retry logic for transient failures
func (d *DB) withRetry(ctx context.Context, operation func() error) error {
	retryConfig := RetryConfig{
		Attempts: d.config.RetryAttempts,
		Delay:    d.config.RetryDelay,
		MaxDelay: d.config.RetryMaxDelay,
	}

	// Set defaults if not configured
	if retryConfig.Attempts == 0 {
		retryConfig.Attempts = 3
	}
	if retryConfig.Delay == 0 {
		retryConfig.Delay = 100 * time.Millisecond
	}
	if retryConfig.MaxDelay == 0 {
		retryConfig.MaxDelay = 5 * time.Second
	}

	var lastErr error

	for attempt := 0; attempt < retryConfig.Attempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				d.logger.Info("operation succeeded after retry",
					slog.Int("attempt", attempt+1),
					slog.Int("total_attempts", retryConfig.Attempts))
			}
			return nil
		}

		lastErr = err

		// Don't retry if it's not a retriable error
		if !isRetriableError(err) {
			d.logger.Debug("error is not retriable, giving up",
				slog.Any("error", err),
				slog.Int("attempt", attempt+1))
			return err
		}

		// Don't sleep on the last attempt
		if attempt < retryConfig.Attempts-1 {
			// Calculate delay with exponential backoff
			delay := time.Duration(float64(retryConfig.Delay) * math.Pow(2, float64(attempt)))
			delay = min(delay, retryConfig.MaxDelay)

			d.logger.Warn("operation failed, retrying",
				slog.Any("error", err),
				slog.Int("attempt", attempt+1),
				slog.Int("total_attempts", retryConfig.Attempts),
				slog.Duration("retry_delay", delay))

			// Sleep with context cancellation support
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		} else {
			d.logger.Error("operation failed after all retry attempts",
				slog.Any("error", err),
				slog.Int("total_attempts", retryConfig.Attempts))
		}
	}

	return NewRetryExhaustedError("database operation", retryConfig.Attempts, lastErr)
}

// Backup creates a database backup using the configured Backuper
func (d *DB) Backup(ctx context.Context) error {
	return d.Backuper.Backup(ctx, d.config)
}

// BackupToFile creates a database backup to a specific file path using the configured Backuper
func (d *DB) BackupToFile(ctx context.Context, filePath string) error {
	return d.Backuper.BackupToFile(ctx, d.config, filePath)
}

// Restore restores a database from a backup file using the configured Restorer
func (d *DB) Restore(ctx context.Context, backupPath string) error {
	return d.Restorer.Restore(ctx, d.config, backupPath)
}
