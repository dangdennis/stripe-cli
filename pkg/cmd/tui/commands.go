package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (m model) executeCommand(resourceName, operationName string) (string, error) {
	// Handle v2 resources by stripping the "v2 " prefix
	cmdArgs := []string{}
	if strings.HasPrefix(resourceName, "v2 ") {
		cmdArgs = append(cmdArgs, "v2", strings.TrimPrefix(resourceName, "v2 "), operationName)
	} else {
		cmdArgs = append(cmdArgs, resourceName, operationName)
	}

	// Find the command in the root command tree
	targetCmd, _, err := m.rootCmd.Find(cmdArgs)
	if err != nil {
		return "", fmt.Errorf("command not found: %v", err)
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer

	// Create a new command context
	ctx := context.Background()

	// Set the output for the command
	originalStdout := targetCmd.OutOrStdout()
	originalStderr := targetCmd.ErrOrStderr()

	targetCmd.SetOut(&stdout)
	targetCmd.SetErr(&stderr)

	// Execute the command
	targetCmd.SetArgs([]string{}) // No additional args for now
	err = targetCmd.ExecuteContext(ctx)

	// Restore original outputs
	targetCmd.SetOut(originalStdout)
	targetCmd.SetErr(originalStderr)

	if err != nil {
		errorOutput := stderr.String()
		if errorOutput == "" {
			errorOutput = err.Error()
		}
		return errorOutput, err
	}

	return m.formatOutput(stdout.String()), nil
}

func (m model) formatOutput(output string) string {
	// Try to pretty-print JSON if possible
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(output), &jsonObj); err == nil {
		if prettyJSON, err := json.MarshalIndent(jsonObj, "", "  "); err == nil {
			return string(prettyJSON)
		}
	}

	// If not JSON or formatting fails, return original output
	return output
}

func (m model) getResourceOperations(resourceName string) []string {
	// Common operations that most resources support
	commonOps := []string{"list", "retrieve", "create", "update", "delete"}

	// Special cases for certain resources
	switch resourceName {
	case "balance":
		return []string{"retrieve"} // Balance is a singleton
	case "events":
		return []string{"list", "retrieve"} // Events are read-only
	case "webhook_endpoints":
		return []string{"list", "retrieve", "create", "update", "delete"}
	case "payment_intents":
		return []string{"list", "retrieve", "create", "update", "confirm", "capture", "cancel"}
	case "charges":
		return []string{"list", "retrieve", "create", "update", "capture"}
	default:
		return commonOps
	}
}
