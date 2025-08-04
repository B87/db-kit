package database

import (
	"context"
	"testing"
	"time"
)

func TestConfigConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "basic config",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "password",
				DBName:   "testdb",
			},
			expected: "host=localhost port=5432 user=postgres password=password dbname=testdb sslmode=disable",
		},
		{
			name: "config with SSL",
			config: Config{
				Host:        "localhost",
				Port:        5432,
				User:        "postgres",
				Password:    "password",
				DBName:      "testdb",
				SSLMode:     "require",
				SSLCert:     "/path/to/cert.pem",
				SSLKey:      "/path/to/key.pem",
				SSLRootCert: "/path/to/ca.pem",
			},
			expected: "host=localhost port=5432 user=postgres password=password dbname=testdb sslmode=require sslcert=/path/to/cert.pem sslkey=/path/to/key.pem sslrootcert=/path/to/ca.pem",
		},
		{
			name: "config with timeouts",
			config: Config{
				Host:             "localhost",
				Port:             5432,
				User:             "postgres",
				Password:         "password",
				DBName:           "testdb",
				ConnectTimeout:   30 * time.Second,
				StatementTimeout: 5 * time.Second,
			},
			expected: "host=localhost port=5432 user=postgres password=password dbname=testdb sslmode=disable connect_timeout=30 statement_timeout=5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.ConnectionString()
			if result != tt.expected {
				t.Errorf("Expected connection string %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewDefaultConfiguration(t *testing.T) {
	// Test that NewDefault creates a valid configuration
	db, err := NewDefault()
	if err != nil {
		t.Skipf("Skipping test due to database connection issue: %v", err)
	}
	defer db.Close()

	// Test that we can ping the database
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.Ping(ctx)
	if err != nil {
		t.Skipf("Skipping test due to database ping failure: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test health check
	err := db.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestDBGetter(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	// Test that DB() returns the underlying *sqlx.DB
	sqlxDB := db.DB()
	if sqlxDB == nil {
		t.Error("DB() returned nil")
	}

	// Test that we can use the returned DB
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sqlxDB.PingContext(ctx)
	if err != nil {
		t.Errorf("Failed to ping via returned DB: %v", err)
	}
}

func TestConnectionPoolConfiguration(t *testing.T) {
	config := Config{
		Host:            "localhost",
		Port:            5432,
		User:            "postgres",
		Password:        "postgres",
		DBName:          "postgres",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
		MigrationsDir:   "../tmp/migrations",
		BackupsDir:      "../tmp",
	}

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection issue: %v", err)
	}
	defer db.Close()

	// Test that connection pool is configured
	sqlxDB := db.DB()

	// Note: sqlx doesn't expose getters for these values, so we can only test
	// that the configuration doesn't cause errors
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = sqlxDB.PingContext(ctx)
	if err != nil {
		t.Errorf("Failed to ping database with pool configuration: %v", err)
	}
}
