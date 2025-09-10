package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type commandResult struct {
	output      string
	method      string
	url         string
	requestBody string
}

func (m model) executeCommand(resourceName, operationName string) (commandResult, error) {
	// Only support GET requests for now
	if !strings.Contains(operationName, "list") && !strings.Contains(operationName, "retrieve") && !strings.Contains(operationName, "get") {
		return commandResult{}, fmt.Errorf("only GET operations (list, retrieve, get) are supported in TUI mode")
	}

	// Build metadata
	method := "GET"

	// Construct API URL
	apiVersion := "v1"
	if strings.HasPrefix(resourceName, "v2 ") {
		apiVersion = "v2"
		resourceName = strings.TrimPrefix(resourceName, "v2 ")
	}

	// Convert resource name to API endpoint
	resourcePath := strings.ReplaceAll(resourceName, " ", "/")
	url := fmt.Sprintf("https://api.stripe.com/%s/%s", apiVersion, resourcePath)

	// Log the HTTP request we're about to make
	if m.logger != nil {
		m.logger.LogAction("making_http_request", map[string]interface{}{
			"resource":  resourceName,
			"operation": operationName,
			"method":    method,
			"url":       url,
		})
	}

	// Get API key from profile
	apiKey, err := m.profile.GetAPIKey(false) // Use test mode for now
	if err != nil {
		if m.logger != nil {
			m.logger.LogError("get_api_key", err, map[string]interface{}{
				"resource":  resourceName,
				"operation": operationName,
			})
		}
		return commandResult{}, fmt.Errorf("failed to get API key: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		if m.logger != nil {
			m.logger.LogError("create_request", err, map[string]interface{}{
				"method": method,
				"url":    url,
			})
		}
		return commandResult{}, fmt.Errorf("failed to create request: %v", err)
	}

	// Set authorization header
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Stripe-TUI/1.0")

	// Make HTTP request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	if m.logger != nil {
		m.logger.LogAction("sending_http_request", map[string]interface{}{
			"url": url,
		})
	}

	resp, err := client.Do(req)
	if err != nil {
		if m.logger != nil {
			m.logger.LogError("http_request", err, map[string]interface{}{
				"method": method,
				"url":    url,
			})
		}
		return commandResult{}, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if m.logger != nil {
			m.logger.LogError("read_response", err, map[string]interface{}{
				"status_code": resp.StatusCode,
				"url":         url,
			})
		}
		return commandResult{}, fmt.Errorf("failed to read response: %v", err)
	}

	// Handle HTTP errors
	if resp.StatusCode >= 400 {
		if m.logger != nil {
			m.logger.LogError("http_error_response", fmt.Errorf("HTTP %d", resp.StatusCode), map[string]interface{}{
				"status_code": resp.StatusCode,
				"url":         url,
				"response":    string(body),
			})
		}
		return commandResult{
			output: fmt.Sprintf("HTTP %d Error:\n%s", resp.StatusCode, string(body)),
			method: method,
			url:    url,
		}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	if m.logger != nil {
		m.logger.LogAction("successful_http_request", map[string]interface{}{
			"status_code":   resp.StatusCode,
			"url":           url,
			"response_size": len(body),
		})
	}

	return commandResult{
		output:      m.formatOutput(string(body)),
		method:      method,
		url:         url,
		requestBody: "", // Empty for GET requests
	}, nil
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
		// Log the error for debugging
		if m.logger != nil {
			m.logger.LogError("get_resource_operations", err, map[string]interface{}{
				"resource_name": resourceName,
			})
		}
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
