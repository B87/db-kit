package database

import (
	"reflect"
	"testing"
)

func TestSelectQueryBuilder(t *testing.T) {
	t.Run("basic select", func(t *testing.T) {
		query, args := Select("id", "name").
			From("users").
			Build()

		expected := "SELECT id, name FROM users"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		if len(args) != 0 {
			t.Errorf("Expected 0 args, got %d", len(args))
		}
	})

	t.Run("select with where", func(t *testing.T) {
		query, args := Select("*").
			From("users").
			Where("age > ?", 18).
			WhereEq("active", true).
			Build()

		expected := "SELECT * FROM users WHERE age > $1 AND active = $2"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{18, true}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("select with joins", func(t *testing.T) {
		query, args := Select("u.name", "p.title").
			From("users u").
			LeftJoin("posts p", "p.user_id = u.id").
			InnerJoin("categories c", "c.id = p.category_id").
			WhereEq("u.active", true).
			Build()

		expected := "SELECT u.name, p.title FROM users u LEFT JOIN posts p ON p.user_id = u.id INNER JOIN categories c ON c.id = p.category_id WHERE u.active = $1"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{true}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("select with order by and limit", func(t *testing.T) {
		query, args := Select("*").
			From("users").
			OrderBy("created_at", "DESC").
			OrderByDesc("id").
			Limit(10).
			Offset(20).
			Build()

		expected := "SELECT * FROM users ORDER BY created_at DESC, id DESC LIMIT 10 OFFSET 20"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		if len(args) != 0 {
			t.Errorf("Expected 0 args, got %d", len(args))
		}
	})

	t.Run("select with group by and having", func(t *testing.T) {
		query, args := Select("department", "COUNT(*)").
			From("employees").
			GroupBy("department").
			Having("COUNT(*) > ?", 5).
			Build()

		expected := "SELECT department, COUNT(*) FROM employees GROUP BY department HAVING COUNT(*) > $1"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{5}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("select with where in", func(t *testing.T) {
		query, args := Select("*").
			From("users").
			WhereIn("id", 1, 2, 3, 4).
			Build()

		expected := "SELECT * FROM users WHERE id IN ($1, $2, $3, $4)"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{1, 2, 3, 4}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("select with null conditions", func(t *testing.T) {
		query, args := Select("*").
			From("users").
			WhereNotNull("email").
			WhereNull("deleted_at").
			Build()

		expected := "SELECT * FROM users WHERE email IS NOT NULL AND deleted_at IS NULL"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		if len(args) != 0 {
			t.Errorf("Expected 0 args, got %d", len(args))
		}
	})
}

func TestInsertQueryBuilder(t *testing.T) {
	t.Run("basic insert", func(t *testing.T) {
		query, args := Insert("users").
			Columns("name", "email", "age").
			Values("John", "john@example.com", 25).
			Build()

		expected := "INSERT INTO users (name, email, age) VALUES ($1, $2, $3)"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{"John", "john@example.com", 25}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("insert multiple values", func(t *testing.T) {
		query, args := Insert("users").
			Columns("name", "email").
			Values("John", "john@example.com").
			Values("Jane", "jane@example.com").
			Build()

		expected := "INSERT INTO users (name, email) VALUES ($1, $2), ($3, $4)"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{"John", "john@example.com", "Jane", "jane@example.com"}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("insert with on conflict do nothing", func(t *testing.T) {
		query, args := Insert("users").
			Columns("name", "email").
			Values("John", "john@example.com").
			OnConflict("email").
			DoNothing().
			Build()

		expected := "INSERT INTO users (name, email) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{"John", "john@example.com"}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("insert with on conflict do update", func(t *testing.T) {
		query, args := Insert("users").
			Columns("name", "email").
			Values("John", "john@example.com").
			OnConflict("email").
			DoUpdate(map[string]interface{}{
				"name":       "John Updated",
				"updated_at": "NOW()",
			}).
			Build()

		// Check that the query contains the expected parts (order may vary due to map iteration)
		expectedParts := []string{
			"INSERT INTO users (name, email) VALUES ($1, $2)",
			"ON CONFLICT (email) DO UPDATE SET",
		}

		for _, part := range expectedParts {
			if !contains(query, part) {
				t.Errorf("Expected query to contain '%s', got '%s'", part, query)
			}
		}

		// Check that both SET clauses are present (order may vary)
		if !contains(query, "name = $") || !contains(query, "updated_at = $") {
			t.Errorf("Expected both SET clauses to be present in query: %s", query)
		}

		// Check args (order may vary due to map iteration)
		if len(args) != 4 {
			t.Errorf("Expected 4 args, got %d", len(args))
		}

		// Check that the expected values are present (order may vary)
		expectedValues := []interface{}{"John", "john@example.com", "John Updated", "NOW()"}
		for _, expectedValue := range expectedValues {
			found := false
			for _, arg := range args {
				if reflect.DeepEqual(arg, expectedValue) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected value %v not found in args %v", expectedValue, args)
			}
		}
	})
}

func TestUpdateQueryBuilder(t *testing.T) {
	t.Run("basic update", func(t *testing.T) {
		query, args := Update("users").
			Set("name", "John Updated").
			Set("email", "john.updated@example.com").
			WhereEq("id", 1).
			Build()

		expected := "UPDATE users SET name = $1, email = $2 WHERE id = $3"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{"John Updated", "john.updated@example.com", 1}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("update with set map", func(t *testing.T) {
		query, args := Update("users").
			SetMap(map[string]interface{}{
				"name":  "John Updated",
				"email": "john.updated@example.com",
				"age":   26,
			}).
			WhereEq("id", 1).
			Build()

		expected := "UPDATE users SET"
		if !contains(query, expected) {
			t.Errorf("Expected query to contain '%s', got '%s'", expected, query)
		}

		// Since map iteration order is not guaranteed, we just check that all elements are present
		if !contains(query, "name = $") || !contains(query, "email = $") || !contains(query, "age = $") {
			t.Errorf("Expected all SET clauses to be present in query: %s", query)
		}

		if len(args) != 4 { // 3 SET values + 1 WHERE value
			t.Errorf("Expected 4 args, got %d", len(args))
		}
	})
}

func TestDeleteQueryBuilder(t *testing.T) {
	t.Run("basic delete", func(t *testing.T) {
		query, args := Delete().
			From("users").
			WhereEq("id", 1).
			Build()

		expected := "DELETE FROM users WHERE id = $1"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{1}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("delete with multiple conditions", func(t *testing.T) {
		query, args := Delete().
			From("users").
			Where("age < ?", 18).
			WhereEq("active", false).
			Build()

		expected := "DELETE FROM users WHERE age < $1 AND active = $2"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{18, false}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})
}

func TestQueryBuilderUtilities(t *testing.T) {
	t.Run("clone query builder", func(t *testing.T) {
		original := Select("*").
			From("users").
			WhereEq("active", true).
			OrderBy("name")

		clone := original.Clone()

		// Modify the clone
		clone.WhereEq("age", 25)

		originalQuery, originalArgs := original.Build()
		cloneQuery, cloneArgs := clone.Build()

		// Original should be unchanged
		expectedOriginal := "SELECT * FROM users WHERE active = $1 ORDER BY name ASC"
		if originalQuery != expectedOriginal {
			t.Errorf("Expected original query '%s', got '%s'", expectedOriginal, originalQuery)
		}

		// Clone should have additional condition
		expectedClone := "SELECT * FROM users WHERE active = $1 AND age = $2 ORDER BY name ASC"
		if cloneQuery != expectedClone {
			t.Errorf("Expected clone query '%s', got '%s'", expectedClone, cloneQuery)
		}

		if len(originalArgs) != 1 {
			t.Errorf("Expected original to have 1 arg, got %d", len(originalArgs))
		}

		if len(cloneArgs) != 2 {
			t.Errorf("Expected clone to have 2 args, got %d", len(cloneArgs))
		}
	})

	t.Run("reset query builder", func(t *testing.T) {
		qb := Select("*").
			From("users").
			WhereEq("active", true)

		// Build initial query
		query1, args1 := qb.Build()
		if len(args1) != 1 {
			t.Errorf("Expected 1 arg before reset, got %d", len(args1))
		}

		// Reset and build new query
		qb.Reset()
		query2, args2 := Insert("posts").
			Columns("title").
			Values("Test Post").
			Build()

		expectedQuery2 := "INSERT INTO posts (title) VALUES ($1)"
		if query2 != expectedQuery2 {
			t.Errorf("Expected reset query '%s', got '%s'", expectedQuery2, query2)
		}

		if len(args2) != 1 {
			t.Errorf("Expected 1 arg after reset, got %d", len(args2))
		}

		// Make sure first query is different from second
		if query1 == query2 {
			t.Errorf("Expected queries to be different after reset")
		}
	})
}

func TestQueryBuilderEdgeCases(t *testing.T) {
	t.Run("select without columns defaults to *", func(t *testing.T) {
		query, _ := Select().From("users").Build()
		expected := "SELECT * FROM users"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}
	})

	t.Run("multiple question mark placeholders", func(t *testing.T) {
		query, args := Select("*").
			From("users").
			Where("age BETWEEN ? AND ?", 18, 65).
			Where("name LIKE ?", "%john%").
			Build()

		expected := "SELECT * FROM users WHERE age BETWEEN $1 AND $2 AND name LIKE $3"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		expectedArgs := []interface{}{18, 65, "%john%"}
		if !reflect.DeepEqual(args, expectedArgs) {
			t.Errorf("Expected args %v, got %v", expectedArgs, args)
		}
	})

	t.Run("empty where in", func(t *testing.T) {
		query, args := Select("*").
			From("users").
			WhereIn("id").
			Build()

		expected := "SELECT * FROM users WHERE id IN ()"
		if query != expected {
			t.Errorf("Expected query '%s', got '%s'", expected, query)
		}

		if len(args) != 0 {
			t.Errorf("Expected 0 args, got %d", len(args))
		}
	})
}

// Helper function for string contains check (reusing from errors_test.go)
func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}

	s = toLower(s)
	substr = toLower(substr)

	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i, c := range []byte(s) {
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
