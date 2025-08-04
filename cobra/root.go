package cobra

import (
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/b87/db-kit/database"
)

var (
	host       *string
	port       *int
	user       *string
	password   *string
	db         *string
	migrations *string
	backups    *string
)

func newDB() (*database.DB, error) {
	return database.NewDefault()
}

// DBCmd is the root command for the db-kit CLI
var DBCmd = &cobra.Command{
	Use: "db",
	Run: func(cmd *cobra.Command, _ []string) {
		err := cmd.Help()
		if err != nil {
			cmd.PrintErrln(err)
			os.Exit(1)
		}
	},
}

// envOrDefault returns the environment variable value or the default if not set
func envOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func init() {
	// Get default values from environment variables
	defaultHost := envOrDefault("POSTGRES_HOST", "localhost")
	defaultPort, _ := strconv.Atoi(envOrDefault("POSTGRES_PORT", "5432"))
	defaultUser := envOrDefault("POSTGRES_USER", "postgres")
	defaultPassword := envOrDefault("POSTGRES_PASSWORD", "postgres")
	defaultDB := envOrDefault("POSTGRES_DB", "dbkit")
	defaultMigrations := envOrDefault("MIGRATIONS_DIR", "./tmp/migrations")
	defaultBackups := envOrDefault("BACKUPS_DIR", "./tmp/backups")

	host = DBCmd.PersistentFlags().String("host", defaultHost, "postgres host")
	port = DBCmd.PersistentFlags().Int("port", defaultPort, "postgres port")
	user = DBCmd.PersistentFlags().String("user", defaultUser, "postgres user")
	password = DBCmd.PersistentFlags().String("password", defaultPassword, "postgres password")
	db = DBCmd.PersistentFlags().String("db", defaultDB, "postgres database")
	migrations = DBCmd.PersistentFlags().String("migrations", defaultMigrations, "directory to store migrations")
	backups = DBCmd.PersistentFlags().String("backups", defaultBackups, "directory to store backups")
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := DBCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
