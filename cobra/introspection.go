package cobra

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/b87/db-kit/database"
)

func init() {
	DBCmd.AddCommand(introspectionCmd)
	introspectionCmd.AddCommand(schemaCmd)
	introspectionCmd.AddCommand(tablesCmd)
	introspectionCmd.AddCommand(tableCmd)
	introspectionCmd.AddCommand(columnsCmd)
	introspectionCmd.AddCommand(indexesCmd)
	introspectionCmd.AddCommand(constraintsCmd)
	introspectionCmd.AddCommand(relationshipsCmd)
	introspectionCmd.AddCommand(versionCmd)
	introspectionCmd.AddCommand(sizeCmd)

	// Add error handling flags to all introspection commands
	addErrorFlags(introspectionCmd)
	addErrorFlags(schemaCmd)
	addErrorFlags(tablesCmd)
	addErrorFlags(tableCmd)
	addErrorFlags(columnsCmd)
	addErrorFlags(indexesCmd)
	addErrorFlags(constraintsCmd)
	addErrorFlags(relationshipsCmd)
	addErrorFlags(versionCmd)
	addErrorFlags(sizeCmd)
}

var introspectionCmd = &cobra.Command{
	Use:   "introspect",
	Short: "Introspect database schema and metadata",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			cmd.PrintErrln(err)
			os.Exit(1)
		}
	},
}

var schemaCmd = &cobra.Command{
	Use:   "schema [schema_name]",
	Short: "Show database schema information",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		var schema string
		if len(args) > 0 {
			schema = args[0]
		}

		if schema == "" {
			// Get all schemas
			schemas, err := introspection.GetSchemas(ctx)
			if err != nil {
				handleError(cmd, err, "get_schemas")
				return
			}
			handleSuccess(cmd, "Schemas retrieved successfully", map[string]interface{}{
				"schemas": schemas,
			})
		} else {
			// Get tables in specific schema
			tables, err := introspection.GetTables(ctx, schema)
			if err != nil {
				handleError(cmd, err, "get_schema_tables")
				return
			}
			handleSuccess(cmd, fmt.Sprintf("Schema '%s' tables retrieved successfully", schema), map[string]interface{}{
				"schema": schema,
				"tables": tables,
			})
		}
	},
}

var tablesCmd = &cobra.Command{
	Use:   "tables [schema_name]",
	Short: "List all tables in the database",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		var schema string
		if len(args) > 0 {
			schema = args[0]
		}

		tables, err := introspection.GetTables(ctx, schema)
		if err != nil {
			handleError(cmd, err, "get_tables")
			return
		}

		handleSuccess(cmd, "Tables retrieved successfully", map[string]interface{}{
			"schema": schema,
			"tables": tables,
		})
	},
}

var tableCmd = &cobra.Command{
	Use:   "table [schema_name] [table_name]",
	Short: "Show detailed information about a specific table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		schema := args[0]
		tableName := args[1]

		// Check if table exists
		exists, err := introspection.GetTableExists(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "check_table_exists")
			return
		}

		if !exists {
			handleError(cmd, fmt.Errorf("table '%s.%s' does not exist", schema, tableName), "table_not_found")
			return
		}

		// Get table information
		tables, err := introspection.GetTables(ctx, schema)
		if err != nil {
			handleError(cmd, err, "get_table_info")
			return
		}

		// Find the specific table
		var tableInfo *database.TableInfo
		for _, table := range tables {
			if table.Name == tableName {
				tableInfo = &table
				break
			}
		}

		if tableInfo == nil {
			handleError(cmd, fmt.Errorf("table '%s.%s' not found", schema, tableName), "table_not_found")
			return
		}

		handleSuccess(cmd, fmt.Sprintf("Table '%s.%s' information retrieved successfully", schema, tableName), map[string]interface{}{
			"table": tableInfo,
		})
	},
}

var columnsCmd = &cobra.Command{
	Use:   "columns [schema_name] [table_name]",
	Short: "Show columns for a specific table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		schema := args[0]
		tableName := args[1]

		// Check if table exists
		exists, err := introspection.GetTableExists(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "check_table_exists")
			return
		}

		if !exists {
			handleError(cmd, fmt.Errorf("table '%s.%s' does not exist", schema, tableName), "table_not_found")
			return
		}

		// Get columns
		columns, err := introspection.GetTableColumns(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "get_table_columns")
			return
		}

		handleSuccess(cmd, fmt.Sprintf("Columns for table '%s.%s' retrieved successfully", schema, tableName), map[string]interface{}{
			"schema":  schema,
			"table":   tableName,
			"columns": columns,
		})
	},
}

var indexesCmd = &cobra.Command{
	Use:   "indexes [schema_name] [table_name]",
	Short: "Show indexes for a specific table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		schema := args[0]
		tableName := args[1]

		// Check if table exists
		exists, err := introspection.GetTableExists(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "check_table_exists")
			return
		}

		if !exists {
			handleError(cmd, fmt.Errorf("table '%s.%s' does not exist", schema, tableName), "table_not_found")
			return
		}

		// Get indexes
		indexes, err := introspection.GetTableIndexes(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "get_table_indexes")
			return
		}

		handleSuccess(cmd, fmt.Sprintf("Indexes for table '%s.%s' retrieved successfully", schema, tableName), map[string]interface{}{
			"schema":  schema,
			"table":   tableName,
			"indexes": indexes,
		})
	},
}

var constraintsCmd = &cobra.Command{
	Use:   "constraints [schema_name] [table_name]",
	Short: "Show constraints for a specific table",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		schema := args[0]
		tableName := args[1]

		// Check if table exists
		exists, err := introspection.GetTableExists(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "check_table_exists")
			return
		}

		if !exists {
			handleError(cmd, fmt.Errorf("table '%s.%s' does not exist", schema, tableName), "table_not_found")
			return
		}

		// Get constraints
		constraints, err := introspection.GetTableConstraints(ctx, schema, tableName)
		if err != nil {
			handleError(cmd, err, "get_table_constraints")
			return
		}

		handleSuccess(cmd, fmt.Sprintf("Constraints for table '%s.%s' retrieved successfully", schema, tableName), map[string]interface{}{
			"schema":      schema,
			"table":       tableName,
			"constraints": constraints,
		})
	},
}

var relationshipsCmd = &cobra.Command{
	Use:   "relationships [schema_name]",
	Short: "Show foreign key relationships in the database",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		var schema string
		if len(args) > 0 {
			schema = args[0]
		}

		// Get foreign key relationships
		relationships, err := introspection.GetForeignKeyRelationships(ctx, schema)
		if err != nil {
			handleError(cmd, err, "get_foreign_key_relationships")
			return
		}

		handleSuccess(cmd, "Foreign key relationships retrieved successfully", map[string]interface{}{
			"schema":        schema,
			"relationships": relationships,
		})
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show database version information",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		version, err := introspection.GetDatabaseVersion(ctx)
		if err != nil {
			handleError(cmd, err, "get_database_version")
			return
		}

		handleSuccess(cmd, "Database version retrieved successfully", map[string]interface{}{
			"version": version,
		})
	},
}

var sizeCmd = &cobra.Command{
	Use:   "size",
	Short: "Show database size information",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		introspection := db.Introspection()

		size, err := introspection.GetDatabaseSize(ctx)
		if err != nil {
			handleError(cmd, err, "get_database_size")
			return
		}

		// Convert bytes to human readable format
		sizeMB := float64(size) / (1024 * 1024)
		sizeGB := sizeMB / 1024

		handleSuccess(cmd, "Database size retrieved successfully", map[string]interface{}{
			"size_bytes": size,
			"size_mb":    sizeMB,
			"size_gb":    sizeGB,
		})
	},
}
