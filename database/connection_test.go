package database

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestRetryLogic(t *testing.T) {
	// Create a mock logger for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	config := Config{
		Host:          "localhost",
		Port:          5432,
		User:          "postgres",
		Password:      "postgres",
		DBName:        "postgres",
		RetryAttempts: 3,
		RetryDelay:    10 * time.Millisecond,
		RetryMaxDelay: 100 * time.Millisecond,
		Logger:        logger,
		LogLevel:      slog.LevelDebug,
		MigrationsDir: "../tmp/migrations",
		BackupsDir:    "../tmp",
	}

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection issue: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("successful operation", func(t *testing.T) {
		callCount := 0
		err := db.withRetry(ctx, func() error {
			callCount++
			return nil
		})

		if err != nil {
			t.Errorf("Expected successful operation, got error: %v", err)
		}

		if callCount != 1 {
			t.Errorf("Expected operation to be called once, got %d calls", callCount)
		}
	})

	t.Run("non-retriable error", func(t *testing.T) {
		callCount := 0
		nonRetriableErr := errors.New("syntax error")

		err := db.withRetry(ctx, func() error {
			callCount++
			return nonRetriableErr
		})

		if err != nonRetriableErr {
			t.Errorf("Expected non-retriable error to be returned, got: %v", err)
		}

		if callCount != 1 {
			t.Errorf("Expected operation to be called once for non-retriable error, got %d calls", callCount)
		}
	})

	t.Run("retriable error eventual success", func(t *testing.T) {
		callCount := 0
		err := db.withRetry(ctx, func() error {
			callCount++
			if callCount < 3 {
				return context.DeadlineExceeded // Retriable error
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected eventual success, got error: %v", err)
		}

		if callCount != 3 {
			t.Errorf("Expected operation to be called 3 times, got %d calls", callCount)
		}
	})
}

func TestIsRetriableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: true,
		},
		{
			name:     "connection timeout",
			err:      errors.New("connection timeout"),
			expected: true,
		},
		{
			name:     "syntax error",
			err:      errors.New("syntax error in query"),
			expected: false,
		},
		{
			name:     "permission denied",
			err:      errors.New("permission denied"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetriableError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected isRetriableError(%v) = %v, got %v", tt.err, tt.expected, result)
			}
		})
	}
}

func TestConnectionValidation(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("validate healthy connection", func(t *testing.T) {
		err := db.ValidateConnection(ctx)
		if err != nil {
			t.Errorf("Connection validation failed: %v", err)
		}
	})

	t.Run("with validation wrapper", func(t *testing.T) {
		operationCalled := false
		err := db.WithValidation(ctx, func() error {
			operationCalled = true
			return nil
		})

		if err != nil {
			t.Errorf("WithValidation failed: %v", err)
		}

		if !operationCalled {
			t.Error("Operation was not called")
		}
	})
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"debug", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"info", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"warn", slog.LevelWarn},
		{"WARNING", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"error", slog.LevelError},
		{"INVALID", slog.LevelInfo}, // defaults to INFO
		{"", slog.LevelInfo},        // defaults to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			if result != tt.expected {
				t.Errorf("Expected parseLogLevel(%q) = %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}

func TestRetryConfiguration(t *testing.T) {
	config := Config{
		Host:          "localhost",
		Port:          5432,
		User:          "postgres",
		Password:      "postgres",
		DBName:        "postgres",
		RetryAttempts: 5,
		RetryDelay:    50 * time.Millisecond,
		RetryMaxDelay: 500 * time.Millisecond,
		LogLevel:      slog.LevelDebug,
		MigrationsDir: "../tmp/migrations",
		BackupsDir:    "../tmp",
	}

	db, err := New(config)
	if err != nil {
		t.Skipf("Skipping test due to database connection issue: %v", err)
	}
	defer db.Close()

	// Verify configuration is applied
	if db.config.RetryAttempts != 5 {
		t.Errorf("Expected RetryAttempts = 5, got %d", db.config.RetryAttempts)
	}

	if db.config.RetryDelay != 50*time.Millisecond {
		t.Errorf("Expected RetryDelay = 50ms, got %v", db.config.RetryDelay)
	}

	if db.config.RetryMaxDelay != 500*time.Millisecond {
		t.Errorf("Expected RetryMaxDelay = 500ms, got %v", db.config.RetryMaxDelay)
	}
}

func TestPingWithRetry(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("ping with retry", func(t *testing.T) {
		err := db.Ping(ctx)
		if err != nil {
			t.Errorf("Ping with retry failed: %v", err)
		}
	})

	t.Run("ping without retry", func(t *testing.T) {
		err := db.PingNoRetry(ctx)
		if err != nil {
			t.Errorf("Ping without retry failed: %v", err)
		}
	})
}

func TestHealthCheckWithRetry(t *testing.T) {
	// Set up the database
	db, close := tearUp(t)
	defer close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("health check with retry", func(t *testing.T) {
		err := db.HealthCheck(ctx)
		if err != nil {
			t.Errorf("Health check with retry failed: %v", err)
		}
	})

	t.Run("health check without retry", func(t *testing.T) {
		err := db.HealthCheckNoRetry(ctx)
		if err != nil {
			t.Errorf("Health check without retry failed: %v", err)
		}
	})
}

func TestDefaultRetryConfiguration(t *testing.T) {
	// Test NewDefault creates proper retry defaults
	db, err := NewDefault()
	if err != nil {
		t.Skipf("Skipping test due to database connection issue: %v", err)
	}
	defer db.Close()

	// Check that defaults are set
	if db.config.RetryAttempts == 0 {
		t.Error("Expected default RetryAttempts to be set")
	}

	if db.config.RetryDelay == 0 {
		t.Error("Expected default RetryDelay to be set")
	}

	if db.config.RetryMaxDelay == 0 {
		t.Error("Expected default RetryMaxDelay to be set")
	}

	if db.logger == nil {
		t.Error("Expected logger to be set")
	}
}
