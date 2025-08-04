package cobra

import (
	"encoding/json"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/b87/db-kit/database"
)

// ErrorOutput represents the structure of CLI error output
type ErrorOutput struct {
	Error       string                 `json:"error"`
	Code        string                 `json:"code,omitempty"`
	Operation   string                 `json:"operation,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
}

// handleError processes errors and provides structured output based on format preference
func handleError(cmd *cobra.Command, err error, operation string) {
	if err == nil {
		return
	}

	// Check if JSON output is requested
	jsonOutput, _ := cmd.Flags().GetBool("json")
	verbose, _ := cmd.Flags().GetBool("verbose")

	// Extract structured error information
	errorOutput := buildErrorOutput(err, operation)

	if jsonOutput {
		// Output as JSON
		if jsonData, jsonErr := json.MarshalIndent(errorOutput, "", "  "); jsonErr == nil {
			cmd.PrintErrln(string(jsonData))
		} else {
			cmd.PrintErrf("Error: %v\n", err)
		}
	} else {
		// Output as human-readable text
		printHumanError(cmd, errorOutput, verbose)
	}

	os.Exit(1)
}

// buildErrorOutput creates structured error output from an error
func buildErrorOutput(err error, operation string) ErrorOutput {
	output := ErrorOutput{
		Error:     err.Error(),
		Operation: operation,
	}

	// Extract structured information if it's a DBError
	var dbErr *database.DBError
	if errors.As(err, &dbErr) {
		output.Code = string(dbErr.Code)
		output.Context = dbErr.Context
		output.UserMessage = dbErr.UserMessage
		output.Suggestions = getSuggestions(dbErr.Code)
	} else {
		output.Code = string(database.ErrCodeUnknown)
		output.UserMessage = err.Error()
	}

	return output
}

// printHumanError prints error in human-readable format
func printHumanError(cmd *cobra.Command, errorOutput ErrorOutput, verbose bool) {
	// Print user-friendly message first
	if errorOutput.UserMessage != "" {
		cmd.PrintErrf("Error: %s\n", errorOutput.UserMessage)
	} else {
		cmd.PrintErrf("Error: %s\n", errorOutput.Error)
	}

	// Show error code if available
	if errorOutput.Code != "" && errorOutput.Code != string(database.ErrCodeUnknown) {
		cmd.PrintErrf("Error Code: %s\n", errorOutput.Code)
	}

	// Show operation context if verbose
	if verbose && errorOutput.Operation != "" {
		cmd.PrintErrf("Operation: %s\n", errorOutput.Operation)
	}

	// Show context in verbose mode
	if verbose && len(errorOutput.Context) > 0 {
		cmd.PrintErrln("Context:")
		for key, value := range errorOutput.Context {
			cmd.PrintErrf("  %s: %v\n", key, value)
		}
	}

	// Show suggestions if available
	if len(errorOutput.Suggestions) > 0 {
		cmd.PrintErrln("\nSuggestions:")
		for _, suggestion := range errorOutput.Suggestions {
			cmd.PrintErrf("  • %s\n", suggestion)
		}
	}

	// Show technical details in verbose mode
	if verbose {
		cmd.PrintErrf("\nTechnical Details: %s\n", errorOutput.Error)
	}
}

// getSuggestions provides helpful suggestions based on error code
func getSuggestions(code database.ErrorCode) []string {
	switch code {
	case database.ErrCodeConnectionFailed, database.ErrCodeConnectionRefused:
		return []string{
			"Check if the database server is running",
			"Verify the host and port configuration",
			"Ensure the database is accepting connections",
			"Check firewall settings",
		}
	case database.ErrCodeAuthenticationError, database.ErrCodeInvalidCredentials:
		return []string{
			"Verify your username and password",
			"Check if the user has sufficient privileges",
			"Ensure the database exists and is accessible",
		}
	case database.ErrCodeConnectionTimeout:
		return []string{
			"Check network connectivity to the database",
			"Increase connection timeout if the database is slow",
			"Verify the database is not overloaded",
		}
	case database.ErrCodeMigrationFailed:
		return []string{
			"Check the migration files for syntax errors",
			"Verify the database schema state",
			"Review migration dependencies and order",
			"Check database permissions for schema changes",
		}
	case database.ErrCodeBackupFailed:
		return []string{
			"Ensure pg_dump is installed and accessible",
			"Check disk space for the backup destination",
			"Verify write permissions for the backup directory",
			"Check if the database is accessible",
		}
	case database.ErrCodeRestoreFailed:
		return []string{
			"Ensure pg_restore or psql is installed",
			"Verify the backup file exists and is readable",
			"Check if the backup file format is compatible",
			"Ensure the target database is accessible",
		}
	case database.ErrCodeInvalidConfig:
		return []string{
			"Review your database configuration settings",
			"Check environment variables for typos",
			"Verify all required configuration is provided",
			"Check the configuration file format",
		}
	case database.ErrCodeTooManyConnections:
		return []string{
			"Reduce the number of concurrent connections",
			"Configure connection pooling properly",
			"Check database max_connections setting",
			"Consider increasing database connection limits",
		}
	case database.ErrCodeRetryExhausted:
		return []string{
			"Check if the issue persists and try again later",
			"Verify network stability",
			"Consider increasing retry attempts or delays",
			"Check database server health and load",
		}
	default:
		return []string{
			"Check the database server status",
			"Review configuration and network connectivity",
			"Enable verbose mode for more detailed error information",
		}
	}
}

// addErrorFlags adds common error handling flags to commands
func addErrorFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Output errors in JSON format")
	cmd.Flags().Bool("verbose", false, "Show verbose error information")
}

// handleSuccess handles successful operation output
func handleSuccess(cmd *cobra.Command, message string, data map[string]interface{}) {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	if jsonOutput {
		output := map[string]interface{}{
			"success": true,
			"message": message,
		}
		if data != nil {
			output["data"] = data
		}

		if jsonData, err := json.MarshalIndent(output, "", "  "); err == nil {
			cmd.Println(string(jsonData))
		} else {
			cmd.Println(message)
		}
	} else {
		cmd.Println(message)
	}
}
