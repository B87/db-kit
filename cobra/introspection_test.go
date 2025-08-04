package cobra

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntrospectionCommands(t *testing.T) {
	// Test schema command
	t.Run("schema command", func(t *testing.T) {
		cmd := schemaCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "schema [schema_name]", cmd.Use)
		assert.Equal(t, "Show database schema information", cmd.Short)
	})

	// Test tables command
	t.Run("tables command", func(t *testing.T) {
		cmd := tablesCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "tables [schema_name]", cmd.Use)
		assert.Equal(t, "List all tables in the database", cmd.Short)
	})

	// Test table command
	t.Run("table command", func(t *testing.T) {
		cmd := tableCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "table [schema_name] [table_name]", cmd.Use)
		assert.Equal(t, "Show detailed information about a specific table", cmd.Short)
	})

	// Test columns command
	t.Run("columns command", func(t *testing.T) {
		cmd := columnsCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "columns [schema_name] [table_name]", cmd.Use)
		assert.Equal(t, "Show columns for a specific table", cmd.Short)
	})

	// Test indexes command
	t.Run("indexes command", func(t *testing.T) {
		cmd := indexesCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "indexes [schema_name] [table_name]", cmd.Use)
		assert.Equal(t, "Show indexes for a specific table", cmd.Short)
	})

	// Test constraints command
	t.Run("constraints command", func(t *testing.T) {
		cmd := constraintsCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "constraints [schema_name] [table_name]", cmd.Use)
		assert.Equal(t, "Show constraints for a specific table", cmd.Short)
	})

	// Test relationships command
	t.Run("relationships command", func(t *testing.T) {
		cmd := relationshipsCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "relationships [schema_name]", cmd.Use)
		assert.Equal(t, "Show foreign key relationships in the database", cmd.Short)
	})

	// Test version command
	t.Run("version command", func(t *testing.T) {
		cmd := versionCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "version", cmd.Use)
		assert.Equal(t, "Show database version information", cmd.Short)
	})

	// Test size command
	t.Run("size command", func(t *testing.T) {
		cmd := sizeCmd
		assert.NotNil(t, cmd)
		assert.Equal(t, "size", cmd.Use)
		assert.Equal(t, "Show database size information", cmd.Short)
	})
}

func TestIntrospectionCommandIntegration(t *testing.T) {
	// Skip if no database is available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test with a real database connection
	t.Run("database connection", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		db, err := newDB()
		require.NoError(t, err)
		defer db.Close()

		// Test that introspection service can be created
		introspection := db.Introspection()
		assert.NotNil(t, introspection)

		// Test database version retrieval
		version, err := introspection.GetDatabaseVersion(ctx)
		if err == nil {
			assert.NotEmpty(t, version)
			assert.Contains(t, version, "PostgreSQL")
		}

		// Test database size retrieval
		size, err := introspection.GetDatabaseSize(ctx)
		if err == nil {
			assert.GreaterOrEqual(t, size, int64(0))
		}

		// Test schemas retrieval
		schemas, err := introspection.GetSchemas(ctx)
		if err == nil {
			assert.NotNil(t, schemas)
			// Should at least have the public schema
			foundPublic := false
			for _, schema := range schemas {
				if schema == "public" {
					foundPublic = true
					break
				}
			}
			assert.True(t, foundPublic, "public schema should be present")
		}
	})
}

func TestIntrospectionCommandArgs(t *testing.T) {
	// Test schema command args
	t.Run("schema command args", func(t *testing.T) {
		cmd := schemaCmd
		// Should accept 0 or 1 arguments
		assert.NoError(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"public"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "extra"}))
	})

	// Test tables command args
	t.Run("tables command args", func(t *testing.T) {
		cmd := tablesCmd
		// Should accept 0 or 1 arguments
		assert.NoError(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"public"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "extra"}))
	})

	// Test table command args
	t.Run("table command args", func(t *testing.T) {
		cmd := tableCmd
		// Should accept exactly 2 arguments
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.Error(t, cmd.Args(cmd, []string{"public"}))
		assert.NoError(t, cmd.Args(cmd, []string{"public", "test_table"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "test_table", "extra"}))
	})

	// Test columns command args
	t.Run("columns command args", func(t *testing.T) {
		cmd := columnsCmd
		// Should accept exactly 2 arguments
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.Error(t, cmd.Args(cmd, []string{"public"}))
		assert.NoError(t, cmd.Args(cmd, []string{"public", "test_table"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "test_table", "extra"}))
	})

	// Test indexes command args
	t.Run("indexes command args", func(t *testing.T) {
		cmd := indexesCmd
		// Should accept exactly 2 arguments
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.Error(t, cmd.Args(cmd, []string{"public"}))
		assert.NoError(t, cmd.Args(cmd, []string{"public", "test_table"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "test_table", "extra"}))
	})

	// Test constraints command args
	t.Run("constraints command args", func(t *testing.T) {
		cmd := constraintsCmd
		// Should accept exactly 2 arguments
		assert.Error(t, cmd.Args(cmd, []string{}))
		assert.Error(t, cmd.Args(cmd, []string{"public"}))
		assert.NoError(t, cmd.Args(cmd, []string{"public", "test_table"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "test_table", "extra"}))
	})

	// Test relationships command args
	t.Run("relationships command args", func(t *testing.T) {
		cmd := relationshipsCmd
		// Should accept 0 or 1 arguments
		assert.NoError(t, cmd.Args(cmd, []string{}))
		assert.NoError(t, cmd.Args(cmd, []string{"public"}))
		assert.Error(t, cmd.Args(cmd, []string{"public", "extra"}))
	})

	// Test version command args
	t.Run("version command args", func(t *testing.T) {
		cmd := versionCmd
		// Should accept 0 arguments
		assert.NoError(t, cmd.Args(cmd, []string{}))
		assert.Error(t, cmd.Args(cmd, []string{"extra"}))
	})

	// Test size command args
	t.Run("size command args", func(t *testing.T) {
		cmd := sizeCmd
		// Should accept 0 arguments
		assert.NoError(t, cmd.Args(cmd, []string{}))
		assert.Error(t, cmd.Args(cmd, []string{"extra"}))
	})
}

func TestIntrospectionCommandHelp(t *testing.T) {
	// Test that all commands have help text
	commands := []*cobra.Command{
		introspectionCmd,
		schemaCmd,
		tablesCmd,
		tableCmd,
		columnsCmd,
		indexesCmd,
		constraintsCmd,
		relationshipsCmd,
		versionCmd,
		sizeCmd,
	}

	for _, cmd := range commands {
		t.Run(cmd.Use, func(t *testing.T) {
			assert.NotEmpty(t, cmd.Short, "Command should have a short description")
			assert.NotNil(t, cmd.Run, "Command should have a run function")
		})
	}
}
