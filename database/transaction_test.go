package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestWithTransaction(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	db := testDB.CreateTestDB(t)
	defer db.Close()
	defer testDB.CleanupTestTables(t, db)

	ctx := context.Background()

	// Create a test table
	_, err := db.DB().Exec("CREATE TABLE IF NOT EXISTS test_transactions (id SERIAL PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	t.Run("successful transaction", func(t *testing.T) {
		err := db.WithTransaction(ctx, func(tx *Transaction) error {
			_, err := tx.Exec("INSERT INTO test_transactions (name) VALUES ($1)", "test1")
			if err != nil {
				return err
			}

			_, err = tx.Exec("INSERT INTO test_transactions (name) VALUES ($1)", "test2")
			return err
		})

		if err != nil {
			t.Errorf("Expected transaction to succeed, got error: %v", err)
		}

		// Verify data was committed
		var count int
		err = db.DB().Get(&count, "SELECT COUNT(*) FROM test_transactions WHERE name IN ('test1', 'test2')")
		if err != nil {
			t.Fatalf("Failed to count rows: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected 2 rows, got %d", count)
		}
	})

	t.Run("failed transaction rollback", func(t *testing.T) {
		initialCount := 0
		err := db.DB().Get(&initialCount, "SELECT COUNT(*) FROM test_transactions")
		if err != nil {
			t.Fatalf("Failed to get initial count: %v", err)
		}

		err = db.WithTransaction(ctx, func(tx *Transaction) error {
			_, err := tx.Exec("INSERT INTO test_transactions (name) VALUES ($1)", "test3")
			if err != nil {
				return err
			}

			// Force an error to trigger rollback
			return errors.New("intentional error")
		})

		if err == nil {
			t.Errorf("Expected transaction to fail")
		}

		// Verify data was rolled back
		var finalCount int
		err = db.DB().Get(&finalCount, "SELECT COUNT(*) FROM test_transactions")
		if err != nil {
			t.Fatalf("Failed to get final count: %v", err)
		}
		if finalCount != initialCount {
			t.Errorf("Expected count to remain %d, got %d", initialCount, finalCount)
		}
	})

	t.Run("transaction with context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		// Use a channel to signal when the context is cancelled
		done := make(chan struct{})

		err := db.WithTransaction(ctx, func(_ *Transaction) error {
			// Wait for the context to be cancelled (timeout)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-done:
				return nil
			}
		})

		if err == nil {
			t.Errorf("Expected transaction to fail due to context timeout")
		}

		// WithTransaction should return a DBError with TRANSACTION_FAILED code
		// wrapping the original context.DeadlineExceeded error
		var dbErr *DBError
		if !errors.As(err, &dbErr) {
			t.Errorf("Expected DBError, got: %v", err)
		} else if dbErr.Code != ErrCodeTransactionFailed {
			t.Errorf("Expected TRANSACTION_FAILED error code, got: %s", dbErr.Code)
		}

		// The underlying error should be context.DeadlineExceeded
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expected underlying error to be context.DeadlineExceeded, got: %v", errors.Unwrap(err))
		}
	})
}

func TestWithTransactionIsolation(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	db := testDB.CreateTestDB(t)
	defer db.Close()
	defer testDB.CleanupTestTables(t, db)

	ctx := context.Background()

	// Create a test table
	_, err := db.DB().Exec("CREATE TABLE IF NOT EXISTS test_isolation (id SERIAL PRIMARY KEY, value INT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	t.Run("serializable isolation", func(t *testing.T) {
		err := db.WithTransactionIsolation(ctx, sql.LevelSerializable, func(tx *Transaction) error {
			_, err := tx.Exec("INSERT INTO test_isolation (value) VALUES ($1)", 42)
			return err
		})

		if err != nil {
			t.Errorf("Expected serializable transaction to succeed, got error: %v", err)
		}

		// Verify data was committed
		var value int
		err = db.DB().Get(&value, "SELECT value FROM test_isolation WHERE value = 42")
		if err != nil {
			t.Fatalf("Failed to get value: %v", err)
		}
		if value != 42 {
			t.Errorf("Expected value 42, got %d", value)
		}
	})
}

func TestTransactionMethods(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	db := testDB.CreateTestDB(t)
	defer db.Close()
	defer testDB.CleanupTestTables(t, db)

	ctx := context.Background()

	// Create a test table
	_, err := db.DB().Exec("CREATE TABLE IF NOT EXISTS test_methods (id SERIAL PRIMARY KEY, name TEXT, value INT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	t.Run("transaction query methods", func(t *testing.T) {
		err := db.WithTransaction(ctx, func(tx *Transaction) error {
			// Test Exec
			result, err := tx.Exec("INSERT INTO test_methods (name, value) VALUES ($1, $2)", "test", 100)
			if err != nil {
				return err
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected != 1 {
				t.Errorf("Expected 1 row affected, got %d", rowsAffected)
			}

			// Test Get
			var name string
			err = tx.Get(&name, "SELECT name FROM test_methods WHERE value = $1", 100)
			if err != nil {
				return err
			}
			if name != "test" {
				t.Errorf("Expected name 'test', got '%s'", name)
			}

			// Test QueryRow
			var value int
			row := tx.QueryRow("SELECT value FROM test_methods WHERE name = $1", "test")
			err = row.Scan(&value)
			if err != nil {
				return err
			}
			if value != 100 {
				t.Errorf("Expected value 100, got %d", value)
			}

			// Test Select
			type TestRow struct {
				ID    int    `db:"id"`
				Name  string `db:"name"`
				Value int    `db:"value"`
			}
			var rows []TestRow
			err = tx.Select(&rows, "SELECT id, name, value FROM test_methods WHERE name = $1", "test")
			if err != nil {
				return err
			}
			if len(rows) != 1 {
				t.Errorf("Expected 1 row, got %d", len(rows))
			}

			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})

	t.Run("transaction named query methods", func(t *testing.T) {
		err := db.WithTransaction(ctx, func(tx *Transaction) error {
			// Test NamedExec
			type NamedParams struct {
				Name  string `db:"name"`
				Value int    `db:"value"`
			}
			params := NamedParams{Name: "named_test", Value: 200}
			result, err := tx.NamedExec("INSERT INTO test_methods (name, value) VALUES (:name, :value)", params)
			if err != nil {
				return err
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected != 1 {
				t.Errorf("Expected 1 row affected, got %d", rowsAffected)
			}

			// Test NamedQuery
			rows, err := tx.NamedQuery("SELECT id, name, value FROM test_methods WHERE name = :name",
				map[string]interface{}{"name": "named_test"})
			if err != nil {
				return err
			}
			defer rows.Close()

			var count int
			for rows.Next() {
				count++
				var id, value int
				var name string
				err = rows.Scan(&id, &name, &value)
				if err != nil {
					return err
				}
				if name != "named_test" || value != 200 {
					t.Errorf("Unexpected row data: name=%s, value=%d", name, value)
				}
			}
			if count != 1 {
				t.Errorf("Expected 1 row from named query, got %d", count)
			}

			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})

	t.Run("transaction prepared statements", func(t *testing.T) {
		err := db.WithTransaction(ctx, func(tx *Transaction) error {
			// Test Prepare
			stmt, err := tx.Prepare("INSERT INTO test_methods (name, value) VALUES ($1, $2)")
			if err != nil {
				return err
			}
			defer stmt.Close()

			_, err = stmt.Exec("prepared_test", 300)
			if err != nil {
				return err
			}

			// Test Preparex
			stmtx, err := tx.Preparex("SELECT name FROM test_methods WHERE value = $1")
			if err != nil {
				return err
			}
			defer stmtx.Close()

			var name string
			err = stmtx.Get(&name, 300)
			if err != nil {
				return err
			}
			if name != "prepared_test" {
				t.Errorf("Expected name 'prepared_test', got '%s'", name)
			}

			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})
}

func TestTransactionPanic(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	db := testDB.CreateTestDB(t)
	defer db.Close()
	defer testDB.CleanupTestTables(t, db)

	ctx := context.Background()

	// Create a test table
	_, err := db.DB().Exec("CREATE TABLE IF NOT EXISTS test_panic (id SERIAL PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	t.Run("transaction panic handling", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic to be propagated")
			}
		}()

		err := db.WithTransaction(ctx, func(tx *Transaction) error {
			_, err := tx.Exec("INSERT INTO test_panic (name) VALUES ($1)", "panic_test")
			if err != nil {
				return err
			}

			// This should trigger a panic
			panic("test panic")
		})

		// Should not reach here
		t.Errorf("Expected panic, but got error: %v", err)
	})
}

func TestTransactionContext(t *testing.T) {
	testDB := NewTestDatabase(t)
	defer testDB.Close()

	db := testDB.CreateTestDB(t)
	defer db.Close()
	defer testDB.CleanupTestTables(t, db)

	// Create a test table
	_, err := db.DB().Exec("CREATE TABLE IF NOT EXISTS test_context (id SERIAL PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	t.Run("transaction with context methods", func(t *testing.T) {
		ctx := context.Background()

		err := db.WithTransaction(ctx, func(tx *Transaction) error {
			// Test ExecContext
			result, err := tx.ExecContext(ctx, "INSERT INTO test_context (name) VALUES ($1)", "context_test")
			if err != nil {
				return err
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if rowsAffected != 1 {
				t.Errorf("Expected 1 row affected, got %d", rowsAffected)
			}

			// Test GetContext
			var name string
			err = tx.GetContext(ctx, &name, "SELECT name FROM test_context WHERE name = $1", "context_test")
			if err != nil {
				return err
			}
			if name != "context_test" {
				t.Errorf("Expected name 'context_test', got '%s'", name)
			}

			// Test QueryContext
			rows, err := tx.QueryContext(ctx, "SELECT name FROM test_context WHERE name = $1", "context_test")
			if err != nil {
				return err
			}
			defer rows.Close()

			var count int
			for rows.Next() {
				count++
			}
			if count != 1 {
				t.Errorf("Expected 1 row from query context, got %d", count)
			}

			// Test QueryRowContext
			var value string
			row := tx.QueryRowContext(ctx, "SELECT name FROM test_context WHERE name = $1", "context_test")
			err = row.Scan(&value)
			if err != nil {
				return err
			}
			if value != "context_test" {
				t.Errorf("Expected value 'context_test', got '%s'", value)
			}

			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})
}
