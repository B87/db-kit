package cobra

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	createtype = new(string)
)

func init() {
	DBCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(upCmd)
	migrateCmd.AddCommand(downCmd)
	migrateCmd.AddCommand(statusCmd)
	migrateCmd.AddCommand(createCmd)
	migrateCmd.AddCommand(resetCmd)

	createCmd.Flags().StringVarP(createtype, "type", "t", "sql", "Type of the migration")

	// Add error handling flags to all migration commands
	addErrorFlags(migrateCmd)
	addErrorFlags(upCmd)
	addErrorFlags(downCmd)
	addErrorFlags(statusCmd)
	addErrorFlags(createCmd)
	addErrorFlags(resetCmd)
}

var migrateCmd = &cobra.Command{
	Use: "migrate",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			cmd.PrintErrln(err)
			os.Exit(1)
		}
	},
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Migrate the database up",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		err = db.Migrator.Up(ctx)
		if err != nil {
			handleError(cmd, err, "migrate_up")
			return
		}
		handleSuccess(cmd, "Migration up completed successfully", nil)
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Migrate the database down",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		err = db.Migrator.Down(ctx)
		if err != nil {
			handleError(cmd, err, "migrate_down")
			return
		}
		handleSuccess(cmd, "Migration down completed successfully", nil)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		status, err := db.Migrator.Status(ctx)
		if err != nil {
			handleError(cmd, err, "migration_status")
			return
		}

		// Format status information for output
		statusInfo := map[string]interface{}{
			"current_version": status.Current,
			"latest_version":  status.Latest,
			"applied_count":   status.Applied,
			"pending_count":   status.Pending,
			"migrations":      status.Migrations,
		}

		handleSuccess(cmd, "Migration status retrieved successfully", statusInfo)
	},
}

var createCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new migration file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		name := args[0]

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		err = db.Migrator.NewMigration(ctx, name, *createtype)
		if err != nil {
			handleError(cmd, err, "create_migration")
			return
		}
		handleSuccess(cmd, fmt.Sprintf("Migration '%s' created successfully", name), map[string]interface{}{
			"migration_name": name,
			"migration_type": *createtype,
		})
	},
}

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the database (reset all migrations)",
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		err = db.Migrator.Reset(ctx)
		if err != nil {
			handleError(cmd, err, "reset_migrations")
			return
		}
		handleSuccess(cmd, "Database reset completed successfully", nil)
	},
}
