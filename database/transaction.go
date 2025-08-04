package database

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

// Transaction represents a database transaction with helper methods
type Transaction struct {
	tx     *sqlx.Tx
	db     *DB
	logger *slog.Logger
}

// TransactionFunc is a function that executes within a transaction
type TransactionFunc func(tx *Transaction) error

// WithTransaction executes a function within a database transaction
// The transaction is automatically committed if the function returns nil,
// or rolled back if the function returns an error or panics
func (d *DB) WithTransaction(ctx context.Context, fn TransactionFunc) error {
	return d.withRetry(ctx, func() error {
		// Validate connection before starting transaction
		if err := d.ValidateConnection(ctx); err != nil {
			return WrapError(err, ErrCodeConnectionFailed, "with_transaction", "connection validation failed before transaction")
		}

		// Begin transaction
		tx, err := d.db.BeginTxx(ctx, nil)
		if err != nil {
			return WrapError(err, ErrCodeTransactionBegin, "with_transaction", "failed to begin transaction")
		}

		transaction := &Transaction{
			tx:     tx,
			db:     d,
			logger: d.logger,
		}

		// Handle panics by rolling back the transaction
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("transaction panicked, rolling back", slog.Any("panic", r))
				if rollbackErr := tx.Rollback(); rollbackErr != nil {
					d.logger.Error("failed to rollback transaction after panic", slog.Any("error", rollbackErr))
				}
				panic(r) // re-panic
			}
		}()

		// Execute the function
		if err := fn(transaction); err != nil {
			// Rollback on error
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				d.logger.Error("failed to rollback transaction",
					slog.Any("original_error", err),
					slog.Any("rollback_error", rollbackErr))
				// Return the original error, not the rollback error
				// The rollback failure is logged but shouldn't mask the original issue
			}
			return WrapError(err, ErrCodeTransactionFailed, "with_transaction", "transaction function failed")
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return WrapError(err, ErrCodeTransactionCommit, "with_transaction", "failed to commit transaction")
		}

		return nil
	})
}

// WithTransactionIsolation executes a function within a transaction with specific isolation level
func (d *DB) WithTransactionIsolation(ctx context.Context, isolation sql.IsolationLevel, fn TransactionFunc) error {
	return d.withRetry(ctx, func() error {
		// Validate connection before starting transaction
		if err := d.ValidateConnection(ctx); err != nil {
			return WrapError(err, ErrCodeConnectionFailed, "with_transaction_isolation", "connection validation failed before transaction")
		}

		// Begin transaction with isolation level
		tx, err := d.db.BeginTxx(ctx, &sql.TxOptions{
			Isolation: isolation,
		})
		if err != nil {
			return WrapError(err, ErrCodeTransactionBegin, "with_transaction_isolation", "failed to begin transaction with isolation level")
		}

		transaction := &Transaction{
			tx:     tx,
			db:     d,
			logger: d.logger,
		}

		// Handle panics by rolling back the transaction
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("transaction panicked, rolling back", slog.Any("panic", r))
				if rollbackErr := tx.Rollback(); rollbackErr != nil {
					d.logger.Error("failed to rollback transaction after panic", slog.Any("error", rollbackErr))
				}
				panic(r) // re-panic
			}
		}()

		// Execute the function
		if err := fn(transaction); err != nil {
			// Rollback on error
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				d.logger.Error("failed to rollback transaction",
					slog.Any("original_error", err),
					slog.Any("rollback_error", rollbackErr))
				// Return the original error, not the rollback error
				// The rollback failure is logged but shouldn't mask the original issue
			}
			return WrapError(err, ErrCodeTransactionFailed, "with_transaction_isolation", "transaction function failed")
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return WrapError(err, ErrCodeTransactionCommit, "with_transaction_isolation", "failed to commit transaction")
		}

		return nil
	})
}

// Exec executes a query within the transaction
func (t *Transaction) Exec(query string, args ...interface{}) (sql.Result, error) {
	result, err := t.tx.Exec(query, args...)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_exec", "failed to execute query in transaction").
			WithContext("query", query)
	}
	return result, nil
}

// ExecContext executes a query within the transaction with context
func (t *Transaction) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	result, err := t.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_exec_context", "failed to execute query in transaction").
			WithContext("query", query)
	}
	return result, nil
}

// Query executes a query that returns rows within the transaction
func (t *Transaction) Query(query string, args ...interface{}) (*sqlx.Rows, error) {
	rows, err := t.tx.Queryx(query, args...)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_query", "failed to execute query in transaction").
			WithContext("query", query)
	}
	return rows, nil
}

// QueryContext executes a query that returns rows within the transaction with context
func (t *Transaction) QueryContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	rows, err := t.tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_query_context", "failed to execute query in transaction").
			WithContext("query", query)
	}
	return rows, nil
}

// QueryRow executes a query that returns a single row within the transaction
func (t *Transaction) QueryRow(query string, args ...interface{}) *sqlx.Row {
	return t.tx.QueryRowx(query, args...)
}

// QueryRowContext executes a query that returns a single row within the transaction with context
func (t *Transaction) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	return t.tx.QueryRowxContext(ctx, query, args...)
}

// Get scans a single row into dest within the transaction
func (t *Transaction) Get(dest interface{}, query string, args ...interface{}) error {
	err := t.tx.Get(dest, query, args...)
	if err != nil {
		return WrapError(err, ErrCodeQueryFailed, "transaction_get", "failed to get single row in transaction").
			WithContext("query", query)
	}
	return nil
}

// GetContext scans a single row into dest within the transaction with context
func (t *Transaction) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	err := t.tx.GetContext(ctx, dest, query, args...)
	if err != nil {
		return WrapError(err, ErrCodeQueryFailed, "transaction_get_context", "failed to get single row in transaction").
			WithContext("query", query)
	}
	return nil
}

// Select scans multiple rows into dest within the transaction
func (t *Transaction) Select(dest interface{}, query string, args ...interface{}) error {
	err := t.tx.Select(dest, query, args...)
	if err != nil {
		return WrapError(err, ErrCodeQueryFailed, "transaction_select", "failed to select rows in transaction").
			WithContext("query", query)
	}
	return nil
}

// SelectContext scans multiple rows into dest within the transaction with context
func (t *Transaction) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	err := t.tx.SelectContext(ctx, dest, query, args...)
	if err != nil {
		return WrapError(err, ErrCodeQueryFailed, "transaction_select_context", "failed to select rows in transaction").
			WithContext("query", query)
	}
	return nil
}

// NamedExec executes a named query within the transaction
func (t *Transaction) NamedExec(query string, arg interface{}) (sql.Result, error) {
	result, err := t.tx.NamedExec(query, arg)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_named_exec", "failed to execute named query in transaction").
			WithContext("query", query)
	}
	return result, nil
}

// NamedExecContext executes a named query within the transaction with context
func (t *Transaction) NamedExecContext(ctx context.Context, query string, arg interface{}) (sql.Result, error) {
	result, err := t.tx.NamedExecContext(ctx, query, arg)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_named_exec_context", "failed to execute named query in transaction").
			WithContext("query", query)
	}
	return result, nil
}

// NamedQuery executes a named query that returns rows within the transaction
func (t *Transaction) NamedQuery(query string, arg interface{}) (*sqlx.Rows, error) {
	rows, err := t.tx.NamedQuery(query, arg)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_named_query", "failed to execute named query in transaction").
			WithContext("query", query)
	}
	return rows, nil
}

// NamedQueryContext executes a named query that returns rows within the transaction with context
func (t *Transaction) NamedQueryContext(ctx context.Context, query string, arg interface{}) (*sqlx.Rows, error) {
	// sqlx.Tx doesn't have NamedQueryContext, so we'll use a prepared statement approach
	stmt, err := t.tx.PrepareNamedContext(ctx, query)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_named_query_context", "failed to prepare named query in transaction").
			WithContext("query", query)
	}
	defer stmt.Close()

	rows, err := stmt.QueryxContext(ctx, arg)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_named_query_context", "failed to execute named query in transaction").
			WithContext("query", query)
	}
	return rows, nil
}

// Prepare creates a prepared statement within the transaction
func (t *Transaction) Prepare(query string) (*sql.Stmt, error) {
	stmt, err := t.tx.Prepare(query)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_prepare", "failed to prepare statement in transaction").
			WithContext("query", query)
	}
	return stmt, nil
}

// PrepareContext creates a prepared statement within the transaction with context
func (t *Transaction) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := t.tx.PrepareContext(ctx, query)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_prepare_context", "failed to prepare statement in transaction").
			WithContext("query", query)
	}
	return stmt, nil
}

// Preparex creates a prepared statement with sqlx features within the transaction
func (t *Transaction) Preparex(query string) (*sqlx.Stmt, error) {
	stmt, err := t.tx.Preparex(query)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_preparex", "failed to prepare sqlx statement in transaction").
			WithContext("query", query)
	}
	return stmt, nil
}

// PreparexContext creates a prepared statement with sqlx features within the transaction with context
func (t *Transaction) PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error) {
	stmt, err := t.tx.PreparexContext(ctx, query)
	if err != nil {
		return nil, WrapError(err, ErrCodeQueryFailed, "transaction_preparex_context", "failed to prepare sqlx statement in transaction").
			WithContext("query", query)
	}
	return stmt, nil
}

// Rollback manually rolls back the transaction
func (t *Transaction) Rollback() error {
	err := t.tx.Rollback()
	if err != nil {
		return WrapError(err, ErrCodeTransactionRollback, "transaction_rollback", "failed to rollback transaction")
	}
	return nil
}

// Commit manually commits the transaction
func (t *Transaction) Commit() error {
	err := t.tx.Commit()
	if err != nil {
		return WrapError(err, ErrCodeTransactionCommit, "transaction_commit", "failed to commit transaction")
	}
	return nil
}

// Tx returns the underlying *sqlx.Tx for advanced use cases
func (t *Transaction) Tx() *sqlx.Tx {
	return t.tx
}
