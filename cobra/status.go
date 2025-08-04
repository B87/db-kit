package cobra

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/b87/db-kit/database"
)

func init() {
	DBCmd.AddCommand(dbStatusCmd)
	addErrorFlags(dbStatusCmd)
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show database status and health information",
	Long: `Show comprehensive database status information including:
- Connection health and ping status
- Database metadata (version, size, schemas)
- Migration status
- Connection pool statistics`,
	Run: func(cmd *cobra.Command, _ []string) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		db, err := newDB()
		if err != nil {
			handleError(cmd, err, "connect")
			return
		}
		defer db.Close()

		// Get database status information
		status, err := getDatabaseStatus(ctx, db)
		if err != nil {
			handleError(cmd, err, "get_status")
			return
		}

		// Display status information
		displayStatus(cmd, status)
		handleSuccess(cmd, "Database status retrieved successfully", map[string]interface{}{
			"status": status,
		})
	},
}

// DatabaseStatus represents the comprehensive status information for a database connection
type DatabaseStatus struct {
	Connection struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Database string `json:"database"`
		User     string `json:"user"`
		Status   string `json:"status"`
		Ping     string `json:"ping"`
	} `json:"connection"`

	Health struct {
		Overall    string `json:"overall"`
		Connection string `json:"connection"`
		Query      string `json:"query"`
	} `json:"health"`

	Database struct {
		Version string   `json:"version"`
		Size    *int64   `json:"size,omitempty"`
		Schemas []string `json:"schemas"`
	} `json:"database"`

	Migrations struct {
		Status         string `json:"status"`
		Error          string `json:"error,omitempty"`
		CurrentVersion int64  `json:"current_version,omitempty"`
		LatestVersion  int64  `json:"latest_version,omitempty"`
		AppliedCount   int    `json:"applied_count,omitempty"`
		PendingCount   int    `json:"pending_count,omitempty"`
	} `json:"migrations"`

	Pool struct {
		OpenConnections    int `json:"open_connections"`
		InUseConnections   int `json:"in_use_connections"`
		IdleConnections    int `json:"idle_connections"`
		MaxOpenConnections int `json:"max_open_connections"`
	} `json:"pool"`
}

func getDatabaseStatus(ctx context.Context, databaseConn *database.DB) (*DatabaseStatus, error) {
	status := &DatabaseStatus{}

	// Get configuration from database connection
	config := databaseConn.Config()

	// Connection information
	status.Connection.Host = config.Host
	status.Connection.Port = config.Port
	status.Connection.Database = config.DBName
	status.Connection.User = config.User

	// Test connection
	if err := databaseConn.Ping(ctx); err != nil {
		status.Connection.Status = "disconnected"
		status.Connection.Ping = "failed"
		status.Health.Overall = "unhealthy"
		status.Health.Connection = "failed"
		status.Health.Query = "unknown"
	} else {
		status.Connection.Status = "connected"
		status.Connection.Ping = "successful"
		status.Health.Connection = "healthy"

		// Test query execution
		var result int
		if err := databaseConn.DB().GetContext(ctx, &result, "SELECT 1"); err != nil {
			status.Health.Query = "failed"
			status.Health.Overall = "unhealthy"
		} else {
			status.Health.Query = "healthy"
			status.Health.Overall = "healthy"
		}

		// Get database metadata
		introspection := databaseConn.Introspection()

		// Get database version
		if version, err := introspection.GetDatabaseVersion(ctx); err == nil {
			status.Database.Version = version
		}

		// Get database size
		if size, err := introspection.GetDatabaseSize(ctx); err == nil {
			status.Database.Size = &size
		}

		// Get schemas
		if schemas, err := introspection.GetSchemas(ctx); err == nil {
			status.Database.Schemas = schemas
		}

		// Get migration status
		if migrationStatus, err := databaseConn.Migrator.Status(ctx); err != nil {
			status.Migrations.Status = "error"
			status.Migrations.Error = err.Error()
		} else {
			status.Migrations.Status = "available"
			// Add migration details to the status
			if migrationStatus != nil {
				status.Migrations.CurrentVersion = migrationStatus.Current
				status.Migrations.LatestVersion = migrationStatus.Latest
				status.Migrations.AppliedCount = migrationStatus.Applied
				status.Migrations.PendingCount = migrationStatus.Pending
			}
		}

		// Get connection pool statistics
		stats := databaseConn.DB().Stats()
		status.Pool.OpenConnections = stats.OpenConnections
		status.Pool.InUseConnections = stats.InUse
		status.Pool.IdleConnections = stats.Idle
		status.Pool.MaxOpenConnections = stats.MaxOpenConnections
	}

	return status, nil
}

func displayStatus(cmd *cobra.Command, status *DatabaseStatus) {
	cmd.Println("=== Database Status ===")

	// Connection section
	cmd.Println("\n📡 Connection:")
	cmd.Printf("  Host: %s:%d\n", status.Connection.Host, status.Connection.Port)
	cmd.Printf("  Database: %s\n", status.Connection.Database)
	cmd.Printf("  User: %s\n", status.Connection.User)
	cmd.Printf("  Status: %s\n", status.Connection.Status)
	cmd.Printf("  Ping: %s\n", status.Connection.Ping)

	// Health section
	cmd.Println("\n🏥 Health:")
	cmd.Printf("  Overall: %s\n", status.Health.Overall)
	cmd.Printf("  Connection: %s\n", status.Health.Connection)
	cmd.Printf("  Query: %s\n", status.Health.Query)

	// Database section
	cmd.Println("\n🗄️  Database:")
	cmd.Printf("  Version: %s\n", status.Database.Version)
	if status.Database.Size != nil {
		cmd.Printf("  Size: %d bytes\n", *status.Database.Size)
	}
	cmd.Printf("  Schemas: %d\n", len(status.Database.Schemas))
	if len(status.Database.Schemas) > 0 {
		cmd.Printf("  Schema list: %s\n", fmt.Sprintf("%v", status.Database.Schemas))
	}

	// Migrations section
	cmd.Println("\n🔄 Migrations:")
	cmd.Printf("  Status: %s\n", status.Migrations.Status)
	if status.Migrations.Error != "" {
		cmd.Printf("  Error: %s\n", status.Migrations.Error)
	}
	cmd.Printf("  Current Version: %d\n", status.Migrations.CurrentVersion)
	cmd.Printf("  Latest Version: %d\n", status.Migrations.LatestVersion)
	cmd.Printf("  Applied: %d, Pending: %d\n", status.Migrations.AppliedCount, status.Migrations.PendingCount)

	// Pool section
	cmd.Println("\n🏊 Connection Pool:")
	cmd.Printf("  Open Connections: %d\n", status.Pool.OpenConnections)
	cmd.Printf("  In Use: %d\n", status.Pool.InUseConnections)
	cmd.Printf("  Idle: %d\n", status.Pool.IdleConnections)
	cmd.Printf("  Max Open: %d\n", status.Pool.MaxOpenConnections)
}
