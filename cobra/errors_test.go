package cobra

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/b87/db-kit/database"
)

func TestBuildErrorOutput(t *testing.T) {
	t.Run("DBError", func(t *testing.T) {
		dbErr := database.NewConnectionError("connection failed", errors.New("connection refused")).
			WithContext("host", "localhost").
			WithContext("port", 5432).
			WithOperation("test_operation").
			WithUserMessage("User-friendly message")

		output := buildErrorOutput(dbErr, "cli_operation")

		if output.Code != string(database.ErrCodeConnectionFailed) {
			t.Errorf("Expected code %s, got %s", database.ErrCodeConnectionFailed, output.Code)
		}

		if output.Operation != "cli_operation" {
			t.Errorf("Expected operation 'cli_operation', got '%s'", output.Operation)
		}

		if output.UserMessage != "User-friendly message" {
			t.Errorf("Expected user message, got '%s'", output.UserMessage)
		}

		if output.Context["host"] != "localhost" {
			t.Errorf("Expected host context, got %v", output.Context["host"])
		}

		if len(output.Suggestions) == 0 {
			t.Errorf("Expected suggestions for connection error")
		}
	})

	t.Run("regular error", func(t *testing.T) {
		regularErr := errors.New("some error")
		output := buildErrorOutput(regularErr, "operation")

		if output.Code != string(database.ErrCodeUnknown) {
			t.Errorf("Expected unknown code for regular error, got %s", output.Code)
		}

		if output.UserMessage != "some error" {
			t.Errorf("Expected error message as user message, got '%s'", output.UserMessage)
		}
	})
}

func TestGetSuggestions(t *testing.T) {
	testCases := []struct {
		code                database.ErrorCode
		expectedSuggestions int
	}{
		{database.ErrCodeConnectionFailed, 4},
		{database.ErrCodeAuthenticationError, 3},
		{database.ErrCodeConnectionTimeout, 3},
		{database.ErrCodeMigrationFailed, 4},
		{database.ErrCodeBackupFailed, 4},
		{database.ErrCodeRestoreFailed, 4},
		{database.ErrCodeInvalidConfig, 4},
		{database.ErrCodeTooManyConnections, 4},
		{database.ErrCodeRetryExhausted, 4},
		{database.ErrCodeUnknown, 3}, // default suggestions
	}

	for _, tc := range testCases {
		t.Run(string(tc.code), func(t *testing.T) {
			suggestions := getSuggestions(tc.code)
			if len(suggestions) != tc.expectedSuggestions {
				t.Errorf("Expected %d suggestions for %s, got %d", tc.expectedSuggestions, tc.code, len(suggestions))
			}

			// Ensure all suggestions are non-empty
			for i, suggestion := range suggestions {
				if suggestion == "" {
					t.Errorf("Suggestion %d is empty for code %s", i, tc.code)
				}
			}
		})
	}
}

func TestPrintHumanError(t *testing.T) {
	t.Run("basic error output", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)

		errorOutput := ErrorOutput{
			Error:       "technical error",
			Code:        string(database.ErrCodeConnectionFailed),
			Operation:   "test_operation",
			UserMessage: "User-friendly message",
			Suggestions: []string{"Check connection", "Verify settings"},
		}

		printHumanError(cmd, errorOutput, false)

		output := buf.String()
		if !contains(output, "User-friendly message") {
			t.Errorf("Expected user message in output")
		}

		if !contains(output, "CONNECTION_FAILED") {
			t.Errorf("Expected error code in output")
		}

		if !contains(output, "Check connection") {
			t.Errorf("Expected suggestions in output")
		}
	})

	t.Run("verbose error output", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetErr(&buf)

		errorOutput := ErrorOutput{
			Error:     "technical error",
			Operation: "test_operation",
			Context: map[string]interface{}{
				"host": "localhost",
				"port": 5432,
			},
		}

		printHumanError(cmd, errorOutput, true)

		output := buf.String()
		if !contains(output, "Operation: test_operation") {
			t.Errorf("Expected operation in verbose output")
		}

		if !contains(output, "Context:") {
			t.Errorf("Expected context in verbose output")
		}

		if !contains(output, "Technical Details:") {
			t.Errorf("Expected technical details in verbose output")
		}
	})
}

func TestHandleSuccess(t *testing.T) {
	t.Run("plain text output", func(t *testing.T) {
		cmd := &cobra.Command{}
		var buf bytes.Buffer
		cmd.SetOut(&buf)

		handleSuccess(cmd, "Operation successful", nil)

		output := buf.String()
		if output != "Operation successful\n" {
			t.Errorf("Expected plain success message, got '%s'", output)
		}
	})

	t.Run("json output", func(t *testing.T) {
		cmd := &cobra.Command{}
		cmd.Flags().Bool("json", true, "JSON output")
		cmd.Flags().Set("json", "true")

		var buf bytes.Buffer
		cmd.SetOut(&buf)

		data := map[string]interface{}{
			"count": 5,
			"items": []string{"a", "b", "c"},
		}

		handleSuccess(cmd, "Operation successful", data)

		output := buf.String()

		var result map[string]interface{}
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Errorf("Expected valid JSON output, got error: %v", err)
		}

		if result["success"] != true {
			t.Errorf("Expected success=true in JSON output")
		}

		if result["message"] != "Operation successful" {
			t.Errorf("Expected message in JSON output")
		}

		if result["data"] == nil {
			t.Errorf("Expected data in JSON output")
		}
	})
}

func TestAddErrorFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
	}

	addErrorFlags(cmd)

	// Check that flags were added
	if cmd.Flags().Lookup("json") == nil {
		t.Errorf("Expected json flag to be added")
	}

	if cmd.Flags().Lookup("verbose") == nil {
		t.Errorf("Expected verbose flag to be added")
	}

	// Test flag defaults
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		t.Errorf("Expected json flag default to be false")
	}

	verboseFlag, _ := cmd.Flags().GetBool("verbose")
	if verboseFlag {
		t.Errorf("Expected verbose flag default to be false")
	}
}

// Helper function for string contains check (case-insensitive)
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
