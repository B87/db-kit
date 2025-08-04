package database

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value pairs
		if idx := strings.Index(line, "="); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			// Remove quotes if present
			if len(value) >= 2 && (value[0] == '"' && value[len(value)-1] == '"' ||
				value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}

			// Only set if not already set in environment
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}

	return scanner.Err()
}

// findProjectRoot attempts to find the project root directory
func findProjectRoot() string {
	// Start from current directory and walk up
	current, err := os.Getwd()
	if err != nil {
		return "."
	}

	// Look for common project root indicators
	for {
		// Check if this directory contains project root indicators
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, ".test.env")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, ".env")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, "taskfile.yml")); err == nil {
			return current
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			// We've reached the filesystem root
			break
		}
		current = parent
	}

	return "."
}

// loadEnvFileRobust attempts to load .env file from multiple possible locations
func loadEnvFileRobust(filename string) error {
	projectRoot := findProjectRoot()

	// Try multiple possible paths
	paths := []string{
		filename,                             // Direct path
		filepath.Join(projectRoot, filename), // Project root + filename
		filepath.Join(".", filename),         // Current directory
		filepath.Join("..", filename),        // Parent directory
		filepath.Join("../..", filename),     // Grandparent directory
	}

	for _, path := range paths {
		if err := loadEnvFile(path); err == nil {
			return nil // Successfully loaded
		}
	}

	return fmt.Errorf("could not find %s in any of the expected locations", filename)
}

// TestDatabase represents a test database instance
type TestDatabase struct {
	config  Config
	isLocal bool
}

// NewTestDatabase creates a new test database, preferring existing PostgreSQL over embedded
func NewTestDatabase(t *testing.T) *TestDatabase {

	// Try to load .env files if they exist (root first, then test-specific)
	if err := loadEnvFileRobust(".env"); err != nil {
		t.Logf("No .env file found: %v", err)
	}

	// Try to load .test.env files first (preferred for testing)
	if err := loadEnvFileRobust(".test.env"); err != nil {
		t.Logf("No .test.env file found: %v", err)
	}

	// Try different common PostgreSQL configurations
	port, err := strconv.Atoi(os.Getenv("POSTGRES_PORT"))
	if err != nil {
		port = 5432
	}

	// Parse additional configuration from environment
	maxOpenConns, _ := strconv.Atoi(os.Getenv("POSTGRES_MAX_OPEN_CONNS"))
	maxIdleConns, _ := strconv.Atoi(os.Getenv("POSTGRES_MAX_IDLE_CONNS"))
	connMaxLifetime, _ := time.ParseDuration(os.Getenv("POSTGRES_CONN_MAX_LIFETIME"))
	connMaxIdleTime, _ := time.ParseDuration(os.Getenv("POSTGRES_CONN_MAX_IDLE_TIME"))
	connectTimeout, _ := time.ParseDuration(os.Getenv("POSTGRES_CONNECT_TIMEOUT"))
	statementTimeout, _ := time.ParseDuration(os.Getenv("POSTGRES_STATEMENT_TIMEOUT"))
	retryAttempts, _ := strconv.Atoi(os.Getenv("POSTGRES_RETRY_ATTEMPTS"))
	retryDelay, _ := time.ParseDuration(os.Getenv("POSTGRES_RETRY_DELAY"))
	retryMaxDelay, _ := time.ParseDuration(os.Getenv("POSTGRES_RETRY_MAX_DELAY"))

	configs := []Config{
		{
			Host:     os.Getenv("POSTGRES_HOST"),
			Port:     port,
			User:     os.Getenv("POSTGRES_USER"),
			Password: os.Getenv("POSTGRES_PASSWORD"),
			DBName:   os.Getenv("POSTGRES_DB"),

			// SSL Configuration
			SSLMode:     os.Getenv("POSTGRES_SSL_MODE"),
			SSLCert:     os.Getenv("POSTGRES_SSL_CERT"),
			SSLKey:      os.Getenv("POSTGRES_SSL_KEY"),
			SSLRootCert: os.Getenv("POSTGRES_SSL_ROOT_CERT"),

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

			// Application Paths
			MigrationsDir: os.Getenv("MIGRATIONS_DIR"),
			BackupsDir:    os.Getenv("BACKUPS_DIR"),
		},
	}

	for _, config := range configs {
		if testLocalPostgreSQL(config, t) {
			t.Logf("Using existing PostgreSQL at %s:%d with user '%s'", config.Host, config.Port, config.User)
			return &TestDatabase{
				config:  config,
				isLocal: true,
			}
		}
	}

	t.Fatalf("No PostgreSQL instance found. Please ensure PostgreSQL is running on localhost:5432")
	return nil
}

// GetConfig returns the database configuration
func (td *TestDatabase) GetConfig() Config {
	return td.config
}

// Close stops the test database if it's embedded
func (td *TestDatabase) Close() {
	// No cleanup needed for external PostgreSQL
}

// CreateTestDB creates a new DB instance with the test configuration
func (td *TestDatabase) CreateTestDB(t *testing.T) *DB {
	db, err := New(td.config)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	return db
}

// CreateTestDBWithEnv creates a new DB instance using environment variables
func CreateTestDBWithEnv(t *testing.T) *DB {
	// Load environment variables from .test.env file first (preferred for testing)
	if err := loadEnvFileRobust(".test.env"); err != nil {
		t.Logf("No .test.env file found: %v", err)
	}

	// Load environment variables from .env file
	if err := loadEnvFileRobust(".env"); err != nil {
		t.Logf("No .env file found: %v", err)
	}

	// Create configuration from environment variables
	config := Config{
		Host:     os.Getenv("POSTGRES_HOST"),
		Port:     getEnvInt("POSTGRES_PORT", 5432),
		User:     os.Getenv("POSTGRES_USER"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		DBName:   os.Getenv("POSTGRES_DB"),

		// SSL Configuration
		SSLMode:     os.Getenv("POSTGRES_SSL_MODE"),
		SSLCert:     os.Getenv("POSTGRES_SSL_CERT"),
		SSLKey:      os.Getenv("POSTGRES_SSL_KEY"),
		SSLRootCert: os.Getenv("POSTGRES_SSL_ROOT_CERT"),

		// Connection Pool Configuration
		MaxOpenConns:    getEnvInt("POSTGRES_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("POSTGRES_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getEnvDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute),
		ConnMaxIdleTime: getEnvDuration("POSTGRES_CONN_MAX_IDLE_TIME", 1*time.Minute),

		// Connection Timeouts
		ConnectTimeout:   getEnvDuration("POSTGRES_CONNECT_TIMEOUT", 30*time.Second),
		StatementTimeout: getEnvDuration("POSTGRES_STATEMENT_TIMEOUT", 30*time.Second),

		// Retry Configuration
		RetryAttempts: getEnvInt("POSTGRES_RETRY_ATTEMPTS", 3),
		RetryDelay:    getEnvDuration("POSTGRES_RETRY_DELAY", 100*time.Millisecond),
		RetryMaxDelay: getEnvDuration("POSTGRES_RETRY_MAX_DELAY", 5*time.Second),

		// Application Paths
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),
		BackupsDir:    os.Getenv("BACKUPS_DIR"),
	}

	// Validate required fields
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.User == "" {
		config.User = "postgres"
	}
	if config.DBName == "" {
		config.DBName = "postgres"
	}

	// Log the configuration being used (without sensitive data)
	t.Logf("Attempting to connect to database:")
	t.Logf("  Host: %s", config.Host)
	t.Logf("  Port: %d", config.Port)
	t.Logf("  User: %s", config.User)
	t.Logf("  Database: %s", config.DBName)
	t.Logf("  SSL Mode: %s", config.SSLMode)

	db, err := New(config)
	if err != nil {
		t.Logf("Failed to create database with env config: %v", err)
		t.Logf("This is expected if PostgreSQL is not running or not accessible")
		t.Logf("You can:")
		t.Logf("  1. Start PostgreSQL locally")
		t.Logf("  2. Update your .env file with correct connection details")
		t.Logf("  3. Use Docker: docker run --name postgres -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres")
		panic(err) // Re-panic to maintain the expected behavior
	}
	return db
}

// Helper functions for environment variable parsing
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// CleanupTestTables removes test tables created during testing
func (td *TestDatabase) CleanupTestTables(t *testing.T, db *DB) {
	ctx := context.Background()

	// List of test tables to clean up
	testTables := []string{
		"test_users",
		"test_posts",
		"test_transactions",
		"test_methods",
		"test_panic",
		"test_context",
		"test_isolation",
	}

	for _, table := range testTables {
		_, err := db.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			t.Logf("Warning: Failed to drop test table %s: %v", table, err)
		}
	}

	// Clean up test indexes
	testIndexes := []string{
		"idx_test_users_name",
		"idx_test_users_email",
		"idx_test_posts_user_id",
		"idx_test_posts_published",
	}

	for _, index := range testIndexes {
		_, err := db.db.ExecContext(ctx, fmt.Sprintf("DROP INDEX IF EXISTS %s", index))
		if err != nil {
			t.Logf("Warning: Failed to drop test index %s: %v", index, err)
		}
	}
}

// testLocalPostgreSQL tests if a local PostgreSQL instance is available
func testLocalPostgreSQL(config Config, t *testing.T) bool {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.User, config.Password, config.DBName)

	t.Logf("Testing connection to local PostgreSQL: %s", connStr)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Logf("Failed to open connection: %v", err)
		return false
	}
	defer db.Close()

	// Set a short timeout for the connection test
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to ping the database
	if err := db.PingContext(ctx); err != nil {
		t.Logf("Failed to ping database: %v", err)
		return false
	}

	// Try a simple query to ensure it's working
	var version string
	err = db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		t.Logf("Failed to query database: %v", err)
		return false
	}

	t.Logf("Successfully connected to local PostgreSQL: %s", version)
	return true
}
