package database

import (
	"errors"
	"fmt"
)

// ErrorCode represents different types of database errors
type ErrorCode string

const (
	// Connection errors
	ErrCodeConnectionFailed    ErrorCode = "CONNECTION_FAILED"
	ErrCodeConnectionTimeout   ErrorCode = "CONNECTION_TIMEOUT"
	ErrCodeConnectionRefused   ErrorCode = "CONNECTION_REFUSED"
	ErrCodeAuthenticationError ErrorCode = "AUTHENTICATION_ERROR"
	ErrCodeInvalidCredentials  ErrorCode = "INVALID_CREDENTIALS"

	// Configuration errors
	ErrCodeInvalidConfig ErrorCode = "INVALID_CONFIG"
	ErrCodeMissingConfig ErrorCode = "MISSING_CONFIG"

	// Migration errors
	ErrCodeMigrationFailed   ErrorCode = "MIGRATION_FAILED"
	ErrCodeMigrationNotFound ErrorCode = "MIGRATION_NOT_FOUND"
	ErrCodeMigrationConflict ErrorCode = "MIGRATION_CONFLICT"

	// Backup/Restore errors
	ErrCodeBackupFailed      ErrorCode = "BACKUP_FAILED"
	ErrCodeRestoreFailed     ErrorCode = "RESTORE_FAILED"
	ErrCodeInvalidBackupFile ErrorCode = "INVALID_BACKUP_FILE"

	// Query errors
	ErrCodeQueryFailed         ErrorCode = "QUERY_FAILED"
	ErrCodeSyntaxError         ErrorCode = "SYNTAX_ERROR"
	ErrCodeConstraintViolation ErrorCode = "CONSTRAINT_VIOLATION"

	// Resource errors
	ErrCodeInsufficientResources ErrorCode = "INSUFFICIENT_RESOURCES"
	ErrCodeTooManyConnections    ErrorCode = "TOO_MANY_CONNECTIONS"
	ErrCodeDiskFull              ErrorCode = "DISK_FULL"

	// Retry errors
	ErrCodeRetryExhausted   ErrorCode = "RETRY_EXHAUSTED"
	ErrCodeOperationTimeout ErrorCode = "OPERATION_TIMEOUT"

	// Transaction errors
	ErrCodeTransactionBegin    ErrorCode = "TRANSACTION_BEGIN"
	ErrCodeTransactionCommit   ErrorCode = "TRANSACTION_COMMIT"
	ErrCodeTransactionRollback ErrorCode = "TRANSACTION_ROLLBACK"
	ErrCodeTransactionFailed   ErrorCode = "TRANSACTION_FAILED"

	// Generic errors
	ErrCodeUnknown    ErrorCode = "UNKNOWN"
	ErrCodeInternal   ErrorCode = "INTERNAL"
	ErrCodeValidation ErrorCode = "VALIDATION"
)

// DBError represents a structured database error with context
type DBError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Operation   string                 `json:"operation,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Underlying  error                  `json:"-"`
	Retriable   bool                   `json:"retriable"`
	UserMessage string                 `json:"user_message,omitempty"`
}

func (e *DBError) Error() string {
	if e.Operation != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Operation, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DBError) Unwrap() error {
	return e.Underlying
}

func (e *DBError) Is(target error) bool {
	var dbErr *DBError
	if errors.As(target, &dbErr) {
		return e.Code == dbErr.Code
	}
	return false
}

func (e *DBError) WithContext(key string, value interface{}) *DBError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

func (e *DBError) WithOperation(operation string) *DBError {
	e.Operation = operation
	return e
}

func (e *DBError) WithUserMessage(message string) *DBError {
	e.UserMessage = message
	return e
}

// NewDBError creates a new DBError
func NewDBError(code ErrorCode, message string, underlying error) *DBError {
	return &DBError{
		Code:       code,
		Message:    message,
		Underlying: underlying,
		Retriable:  isErrorCodeRetriable(code),
	}
}

// NewConnectionError creates a connection-related error
func NewConnectionError(message string, underlying error) *DBError {
	return NewDBError(ErrCodeConnectionFailed, message, underlying).
		WithUserMessage("Unable to connect to the database. Please check your connection settings.")
}

// NewMigrationError creates a migration-related error
func NewMigrationError(message string, underlying error) *DBError {
	return NewDBError(ErrCodeMigrationFailed, message, underlying).
		WithUserMessage("Database migration failed. Please check the migration files and database state.")
}

// NewBackupError creates a backup-related error
func NewBackupError(message string, underlying error) *DBError {
	return NewDBError(ErrCodeBackupFailed, message, underlying).
		WithUserMessage("Database backup failed. Please check disk space and permissions.")
}

// NewRestoreError creates a restore-related error
func NewRestoreError(message string, underlying error) *DBError {
	return NewDBError(ErrCodeRestoreFailed, message, underlying).
		WithUserMessage("Database restore failed. Please check the backup file and database permissions.")
}

// NewConfigError creates a configuration-related error
func NewConfigError(message string, underlying error) *DBError {
	return NewDBError(ErrCodeInvalidConfig, message, underlying).
		WithUserMessage("Database configuration is invalid. Please check your settings.")
}

// NewValidationError creates a validation-related error
func NewValidationError(message string, underlying error) *DBError {
	return NewDBError(ErrCodeValidation, message, underlying).
		WithUserMessage("Input validation failed. Please check your parameters.")
}

// NewRetryExhaustedError creates a retry exhausted error
func NewRetryExhaustedError(operation string, attempts int, underlying error) *DBError {
	return NewDBError(ErrCodeRetryExhausted,
		fmt.Sprintf("operation failed after %d attempts", attempts), underlying).
		WithOperation(operation).
		WithContext("attempts", attempts).
		WithUserMessage("Operation failed after multiple retry attempts. Please try again later or check your connection.")
}

// isErrorCodeRetriable determines if an error code represents a retriable condition
func isErrorCodeRetriable(code ErrorCode) bool {
	switch code {
	case ErrCodeConnectionTimeout,
		ErrCodeConnectionRefused,
		ErrCodeConnectionFailed,
		ErrCodeInsufficientResources,
		ErrCodeTooManyConnections,
		ErrCodeDiskFull,
		ErrCodeOperationTimeout:
		return true
	default:
		return false
	}
}

// WrapError wraps an existing error with additional context
func WrapError(err error, code ErrorCode, operation string, message string) *DBError {
	if err == nil {
		return nil
	}

	// If it's already a DBError, enhance it
	var dbErr *DBError
	if errors.As(err, &dbErr) {
		if operation != "" {
			dbErr.Operation = operation
		}
		if message != "" {
			dbErr.Message = message + ": " + dbErr.Message
		}
		return dbErr
	}

	// Create new DBError
	return NewDBError(code, message, err).WithOperation(operation)
}

// IsRetriable checks if an error is retriable
func IsRetriable(err error) bool {
	if err == nil {
		return false
	}

	var dbErr *DBError
	if errors.As(err, &dbErr) {
		return dbErr.Retriable
	}

	// Fall back to original isRetriableError logic for non-DBError types
	return isRetriableError(err)
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	if err == nil {
		return ""
	}

	var dbErr *DBError
	if errors.As(err, &dbErr) {
		return dbErr.Code
	}

	return ErrCodeUnknown
}

// GetUserMessage extracts a user-friendly message from an error
func GetUserMessage(err error) string {
	if err == nil {
		return ""
	}

	var dbErr *DBError
	if errors.As(err, &dbErr) {
		if dbErr.UserMessage != "" {
			return dbErr.UserMessage
		}
		return dbErr.Message
	}

	return err.Error()
}
