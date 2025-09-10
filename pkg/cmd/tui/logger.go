package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry
type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelDebug LogLevel = "DEBUG"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Type      string                 `json:"type"` // "state_change", "action", "error"
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// TUILogger handles logging for the TUI
type TUILogger struct {
	file       *os.File
	mu         sync.Mutex
	logPath    string
	maxEntries int
}

// NewTUILogger creates a new TUI logger instance
func NewTUILogger() (*TUILogger, error) {
	// Create logs directory in the same directory as the TUI package
	// Get the directory where this file is located
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("failed to get current file path")
	}

	logDir := filepath.Join(filepath.Dir(currentFile), "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02")
	logPath := filepath.Join(logDir, fmt.Sprintf("tui-%s.log", timestamp))

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger := &TUILogger{
		file:       file,
		logPath:    logPath,
		maxEntries: 10000, // Rotate after 10k entries per day
	}

	// Log session start
	logger.logEntry(LogLevelInfo, "session", "TUI session started", nil)

	return logger, nil
}

// Close closes the logger and cleans up resources
func (l *TUILogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.logEntry(LogLevelInfo, "session", "TUI session ended", nil)
		l.file.Close()
		l.file = nil
	}
}

// logEntry writes a log entry to the file
func (l *TUILogger) logEntry(level LogLevel, logType, message string, data map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Type:      logType,
		Message:   message,
		Data:      data,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		// Fallback to simple format if JSON marshaling fails
		fmt.Fprintf(l.file, "[%s] %s %s: %s\n",
			entry.Timestamp.Format("2006-01-02 15:04:05.000"),
			level, logType, message)
		return
	}

	l.file.Write(jsonData)
	l.file.WriteString("\n")
	l.file.Sync() // Ensure data is written immediately
}

// LogStateChange logs when the TUI state changes
func (l *TUILogger) LogStateChange(oldState, newState string, details map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := map[string]interface{}{
		"old_state": oldState,
		"new_state": newState,
	}

	// Merge additional details
	for k, v := range details {
		data[k] = v
	}

	l.logEntry(LogLevelInfo, "state_change",
		fmt.Sprintf("State transition: %s -> %s", oldState, newState), data)
}

// LogAction logs user actions and interactions
func (l *TUILogger) LogAction(action string, details map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.logEntry(LogLevelInfo, "action", action, details)
}

// LogError logs errors that occur during TUI operations
func (l *TUILogger) LogError(operation string, err error, details map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := map[string]interface{}{
		"operation": operation,
		"error":     err.Error(),
	}

	// Merge additional details
	for k, v := range details {
		data[k] = v
	}

	l.logEntry(LogLevelError, "error",
		fmt.Sprintf("Error in %s: %v", operation, err), data)
}

// LogCommand logs command executions
func (l *TUILogger) LogCommand(command string, result *commandResult, err error, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := map[string]interface{}{
		"command":  command,
		"duration": duration.String(),
	}

	if result != nil {
		data["method"] = result.method
		data["url"] = result.url
		data["output_length"] = len(result.output)
		if result.requestBody != "" {
			data["request_body_length"] = len(result.requestBody)
		}
	}

	level := LogLevelInfo
	message := fmt.Sprintf("Command executed: %s (took %v)", command, duration)

	if err != nil {
		level = LogLevelError
		message = fmt.Sprintf("Command failed: %s (took %v)", command, duration)
		data["error"] = err.Error()
	}

	l.logEntry(level, "command", message, data)
}

// LogKeyPress logs key press events for debugging interaction flows
func (l *TUILogger) LogKeyPress(key string, activeList int, context map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := map[string]interface{}{
		"key":         key,
		"active_list": activeList,
	}

	// Merge additional context
	for k, v := range context {
		data[k] = v
	}

	l.logEntry(LogLevelDebug, "keypress", fmt.Sprintf("Key pressed: %s", key), data)
}

// LogListSelection logs when user selects different items in lists
func (l *TUILogger) LogListSelection(listType string, selectedItem string, index int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := map[string]interface{}{
		"list_type":     listType,
		"selected_item": selectedItem,
		"index":         index,
	}

	l.logEntry(LogLevelInfo, "selection",
		fmt.Sprintf("Selected %s in %s list", selectedItem, listType), data)
}

// LogViewChange logs when the active view/panel changes
func (l *TUILogger) LogViewChange(oldView, newView string, trigger string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	data := map[string]interface{}{
		"old_view": oldView,
		"new_view": newView,
		"trigger":  trigger,
	}

	l.logEntry(LogLevelInfo, "view_change",
		fmt.Sprintf("View changed: %s -> %s (trigger: %s)", oldView, newView, trigger), data)
}

// GetLogPath returns the current log file path
func (l *TUILogger) GetLogPath() string {
	return l.logPath
}

// Helper function to get view name from active list index
func getViewName(activeList int) string {
	switch activeList {
	case 0:
		return "resources"
	case 1:
		return "operations"
	case 2:
		return "history"
	default:
		return "unknown"
	}
}
