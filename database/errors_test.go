package database

import (
	"errors"
	"testing"
)

func TestDBError(t *testing.T) {
	t.Run("basic error creation", func(t *testing.T) {
		underlying := errors.New("connection refused")
		err := NewDBError(ErrCodeConnectionFailed, "failed to connect", underlying)

		if err.Code != ErrCodeConnectionFailed {
			t.Errorf("Expected code %s, got %s", ErrCodeConnectionFailed, err.Code)
		}

		if err.Message != "failed to connect" {
			t.Errorf("Expected message 'failed to connect', got '%s'", err.Message)
		}

		if err.Underlying != underlying {
			t.Errorf("Expected underlying error to be preserved")
		}

		if !err.Retriable {
			t.Errorf("Expected connection failed error to be retriable")
		}
	})

	t.Run("error string representation", func(t *testing.T) {
		err := NewDBError(ErrCodeConnectionFailed, "failed to connect", nil)
		expected := "[CONNECTION_FAILED] failed to connect"
		if err.Error() != expected {
			t.Errorf("Expected error string '%s', got '%s'", expected, err.Error())
		}

		err = err.WithOperation("ping")
		expected = "[CONNECTION_FAILED] ping: failed to connect"
		if err.Error() != expected {
			t.Errorf("Expected error string with operation '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("error with context", func(t *testing.T) {
		err := NewDBError(ErrCodeConnectionFailed, "failed to connect", nil).
			WithContext("host", "localhost").
			WithContext("port", 5432)

		if err.Context["host"] != "localhost" {
			t.Errorf("Expected host context to be 'localhost', got %v", err.Context["host"])
		}

		if err.Context["port"] != 5432 {
			t.Errorf("Expected port context to be 5432, got %v", err.Context["port"])
		}
	})

	t.Run("error unwrapping", func(t *testing.T) {
		underlying := errors.New("connection refused")
		err := NewDBError(ErrCodeConnectionFailed, "failed to connect", underlying)

		unwrapped := errors.Unwrap(err)
		if unwrapped != underlying {
			t.Errorf("Expected unwrapped error to be the original error")
		}
	})

	t.Run("error comparison", func(t *testing.T) {
		err1 := NewDBError(ErrCodeConnectionFailed, "failed to connect", nil)
		err2 := NewDBError(ErrCodeConnectionFailed, "another connection error", nil)
		err3 := NewDBError(ErrCodeMigrationFailed, "migration error", nil)

		if !errors.Is(err1, err2) {
			t.Errorf("Expected errors with same code to be equal")
		}

		if errors.Is(err1, err3) {
			t.Errorf("Expected errors with different codes to not be equal")
		}
	})
}

func TestSpecificErrorConstructors(t *testing.T) {
	t.Run("connection error", func(t *testing.T) {
		underlying := errors.New("connection refused")
		err := NewConnectionError("failed to connect", underlying)

		if err.Code != ErrCodeConnectionFailed {
			t.Errorf("Expected connection error code, got %s", err.Code)
		}

		if err.UserMessage == "" {
			t.Errorf("Expected user message to be set")
		}
	})

	t.Run("migration error", func(t *testing.T) {
		underlying := errors.New("syntax error")
		err := NewMigrationError("migration failed", underlying)

		if err.Code != ErrCodeMigrationFailed {
			t.Errorf("Expected migration error code, got %s", err.Code)
		}

		if err.UserMessage == "" {
			t.Errorf("Expected user message to be set")
		}
	})

	t.Run("backup error", func(t *testing.T) {
		underlying := errors.New("disk full")
		err := NewBackupError("backup failed", underlying)

		if err.Code != ErrCodeBackupFailed {
			t.Errorf("Expected backup error code, got %s", err.Code)
		}
	})

	t.Run("restore error", func(t *testing.T) {
		underlying := errors.New("file not found")
		err := NewRestoreError("restore failed", underlying)

		if err.Code != ErrCodeRestoreFailed {
			t.Errorf("Expected restore error code, got %s", err.Code)
		}
	})

	t.Run("config error", func(t *testing.T) {
		underlying := errors.New("invalid port")
		err := NewConfigError("config error", underlying)

		if err.Code != ErrCodeInvalidConfig {
			t.Errorf("Expected config error code, got %s", err.Code)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		underlying := errors.New("required field missing")
		err := NewValidationError("validation failed", underlying)

		if err.Code != ErrCodeValidation {
			t.Errorf("Expected validation error code, got %s", err.Code)
		}
	})

	t.Run("retry exhausted error", func(t *testing.T) {
		underlying := errors.New("operation failed")
		err := NewRetryExhaustedError("test_operation", 3, underlying)

		if err.Code != ErrCodeRetryExhausted {
			t.Errorf("Expected retry exhausted error code, got %s", err.Code)
		}

		if err.Operation != "test_operation" {
			t.Errorf("Expected operation to be 'test_operation', got '%s'", err.Operation)
		}

		if err.Context["attempts"] != 3 {
			t.Errorf("Expected attempts context to be 3, got %v", err.Context["attempts"])
		}
	})
}

func TestErrorCodeRetriability(t *testing.T) {
	retriableTests := []struct {
		code      ErrorCode
		retriable bool
	}{
		{ErrCodeConnectionFailed, true},
		{ErrCodeConnectionTimeout, true},
		{ErrCodeConnectionRefused, true},
		{ErrCodeInsufficientResources, true},
		{ErrCodeTooManyConnections, true},
		{ErrCodeDiskFull, true},
		{ErrCodeOperationTimeout, true},
		{ErrCodeAuthenticationError, false},
		{ErrCodeSyntaxError, false},
		{ErrCodeConstraintViolation, false},
		{ErrCodeInvalidConfig, false},
		{ErrCodeUnknown, false},
	}

	for _, test := range retriableTests {
		t.Run(string(test.code), func(t *testing.T) {
			retriable := isErrorCodeRetriable(test.code)
			if retriable != test.retriable {
				t.Errorf("Expected code %s retriable=%t, got %t", test.code, test.retriable, retriable)
			}
		})
	}
}

func TestWrapError(t *testing.T) {
	t.Run("wrap regular error", func(t *testing.T) {
		original := errors.New("original error")
		wrapped := WrapError(original, ErrCodeConnectionFailed, "test_operation", "wrapped message")

		if wrapped.Code != ErrCodeConnectionFailed {
			t.Errorf("Expected wrapped error code, got %s", wrapped.Code)
		}

		if wrapped.Operation != "test_operation" {
			t.Errorf("Expected operation to be 'test_operation', got '%s'", wrapped.Operation)
		}

		if wrapped.Underlying != original {
			t.Errorf("Expected underlying error to be preserved")
		}
	})

	t.Run("wrap DBError", func(t *testing.T) {
		original := NewDBError(ErrCodeMigrationFailed, "original message", nil)
		wrapped := WrapError(original, ErrCodeConnectionFailed, "new_operation", "wrapped message")

		// Should enhance the existing DBError
		if wrapped.Operation != "new_operation" {
			t.Errorf("Expected operation to be updated to 'new_operation', got '%s'", wrapped.Operation)
		}

		expectedMessage := "wrapped message: original message"
		if wrapped.Message != expectedMessage {
			t.Errorf("Expected message '%s', got '%s'", expectedMessage, wrapped.Message)
		}
	})

	t.Run("wrap nil error", func(t *testing.T) {
		wrapped := WrapError(nil, ErrCodeConnectionFailed, "test", "message")
		if wrapped != nil {
			t.Errorf("Expected nil when wrapping nil error")
		}
	})
}

func TestIsRetriable(t *testing.T) {
	t.Run("retriable DBError", func(t *testing.T) {
		err := NewDBError(ErrCodeConnectionFailed, "connection failed", nil)
		if !IsRetriable(err) {
			t.Errorf("Expected connection failed error to be retriable")
		}
	})

	t.Run("non-retriable DBError", func(t *testing.T) {
		err := NewDBError(ErrCodeSyntaxError, "syntax error", nil)
		if IsRetriable(err) {
			t.Errorf("Expected syntax error to not be retriable")
		}
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("connection refused")
		// Should fall back to original isRetriableError logic
		result := IsRetriable(err)
		expected := isRetriableError(err)
		if result != expected {
			t.Errorf("Expected fallback to original logic")
		}
	})

	t.Run("nil error", func(t *testing.T) {
		if IsRetriable(nil) {
			t.Errorf("Expected nil error to not be retriable")
		}
	})
}

func TestGetErrorCode(t *testing.T) {
	t.Run("DBError", func(t *testing.T) {
		err := NewDBError(ErrCodeConnectionFailed, "connection failed", nil)
		code := GetErrorCode(err)
		if code != ErrCodeConnectionFailed {
			t.Errorf("Expected code %s, got %s", ErrCodeConnectionFailed, code)
		}
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("regular error")
		code := GetErrorCode(err)
		if code != ErrCodeUnknown {
			t.Errorf("Expected unknown code for regular error, got %s", code)
		}
	})

	t.Run("nil error", func(t *testing.T) {
		code := GetErrorCode(nil)
		if code != "" {
			t.Errorf("Expected empty code for nil error, got %s", code)
		}
	})
}

func TestGetUserMessage(t *testing.T) {
	t.Run("DBError with user message", func(t *testing.T) {
		err := NewDBError(ErrCodeConnectionFailed, "technical message", nil).
			WithUserMessage("User-friendly message")

		message := GetUserMessage(err)
		if message != "User-friendly message" {
			t.Errorf("Expected user message, got '%s'", message)
		}
	})

	t.Run("DBError without user message", func(t *testing.T) {
		err := NewDBError(ErrCodeConnectionFailed, "technical message", nil)

		message := GetUserMessage(err)
		if message != "technical message" {
			t.Errorf("Expected technical message as fallback, got '%s'", message)
		}
	})

	t.Run("regular error", func(t *testing.T) {
		err := errors.New("regular error")

		message := GetUserMessage(err)
		if message != "regular error" {
			t.Errorf("Expected error message, got '%s'", message)
		}
	})

	t.Run("nil error", func(t *testing.T) {
		message := GetUserMessage(nil)
		if message != "" {
			t.Errorf("Expected empty message for nil error, got '%s'", message)
		}
	})
}
