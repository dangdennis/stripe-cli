package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (m model) executeCommand(resourceName, operationName string) (string, error) {
	// Parse resource name to build command path
	cmdArgs := []string{}

	if strings.HasPrefix(resourceName, "v2 ") {
		// Handle V2 resources: "v2 namespace resource" or "v2 resource"
		parts := strings.Fields(strings.TrimPrefix(resourceName, "v2 "))
		cmdArgs = append(cmdArgs, "v2")
		cmdArgs = append(cmdArgs, parts...)
		cmdArgs = append(cmdArgs, operationName)
	} else {
		// Handle V1 resources: "namespace resource" or "resource"
		parts := strings.Fields(resourceName)
		cmdArgs = append(cmdArgs, parts...)
		cmdArgs = append(cmdArgs, operationName)
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
	// Parse resource name to build command path
	var targetCmd *cobra.Command
	var err error

	if strings.HasPrefix(resourceName, "v2 ") {
		// Handle V2 resources: "v2 namespace resource" or "v2 resource"
		parts := strings.Fields(strings.TrimPrefix(resourceName, "v2 "))
		cmdArgs := append([]string{"v2"}, parts...)
		targetCmd, _, err = m.rootCmd.Find(cmdArgs)
	} else {
		// Handle V1 resources: "namespace resource" or "resource"
		parts := strings.Fields(resourceName)
		targetCmd, _, err = m.rootCmd.Find(parts)
	}

	if err != nil {
		// Fallback to common operations if command not found
		return []string{}
	}

	// Get actual operations from the command's subcommands
	operations := []string{}
	for _, subCmd := range targetCmd.Commands() {
		if !subCmd.Hidden {
			operations = append(operations, subCmd.Name())
		}
	}

	// If no subcommands found, return common operations as fallback
	if len(operations) == 0 {
		return []string{}
	}

	return operations
}
