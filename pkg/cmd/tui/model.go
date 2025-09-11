package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/config"
)

type resourceListItem struct {
	title        string
	description  string
	resourceType string
}

func (i resourceListItem) FilterValue() string { return i.title }
func (i resourceListItem) Title() string       { return i.title }
func (i resourceListItem) Description() string { return i.description }

type animTickMsg struct{}

func doAnimTick() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return animTickMsg{}
	})
}

type responseHistoryEntry struct {
	command     string
	timestamp   time.Time
	response    string
	error       string
	method      string
	url         string
	requestBody string
}

// The model reflects the entire application state.
type model struct {
	resourceList     list.Model
	operationList    list.Model
	responseHistory  list.Model
	choice           string
	commandOutput    string
	quitting         bool
	activeList       int // 0 for resource list, 1 for operation list, 2 for response history, 3 for output console
	width            int
	height           int
	rootCmd          *cobra.Command
	profile          *config.Profile
	livemode         bool
	showOutput       bool
	outputScroll     int
	showWelcome      bool
	animFrame        int
	historyEntries   []responseHistoryEntry
	selectedResponse int
	logger           *TUILogger
	// Filter state
	filterMode        bool
	filterText        string
	allResourceItems  []list.Item
	allOperationItems []list.Item
}

func (m model) Init() tea.Cmd {
	if m.showWelcome {
		return doAnimTick()
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case animTickMsg:
		return m.handleAnimTick()
	case tea.WindowSizeMsg:
		return m.handleWindowResize(msg)
	case tea.KeyMsg:
		// Handle special keys first
		if newModel, cmd, handled := m.handleSpecialKeys(msg); handled {
			return newModel, cmd
		}
		// If not handled, fall through to list updates
	}

	return m.handleListUpdates(msg)
}

func (m model) updateOperationsList(resourceName string) model {
	// Get operations for the selected resource
	operations := m.getResourceOperations(resourceName)

	// Create operation items
	operationItems := make([]list.Item, 0, len(operations))
	for _, op := range operations {
		operationItems = append(operationItems, resourceListItem{
			title:       op,
			description: fmt.Sprintf("%s operation", op),
		})
	}

	// Store all operation items for filtering
	m.allOperationItems = operationItems

	// Update the operations list
	m.operationList.SetItems(operationItems)
	return m
}

func (m model) handleAnimTick() (tea.Model, tea.Cmd) {
	if m.showWelcome {
		m.animFrame = (m.animFrame + 1) % 20
		return m, doAnimTick()
	}
	return m, nil
}

func (m model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	listWidth := (msg.Width - 4) / 2
	m.resourceList.SetWidth(listWidth)
	m.operationList.SetWidth(listWidth)

	// History panel gets narrow width (left side of bottom area)
	historyWidth := 25
	if historyWidth > msg.Width/4 {
		historyWidth = msg.Width / 4
	}
	m.responseHistory.SetWidth(historyWidth - 2) // Account for borders

	// Calculate heights - half and half split
	bottomHeight := msg.Height / 2
	if bottomHeight < 8 {
		bottomHeight = 8
	}
	listHeight := msg.Height - 6 - bottomHeight // Account for preview line and borders

	m.resourceList.SetHeight(listHeight)
	m.operationList.SetHeight(listHeight)
	m.responseHistory.SetHeight(bottomHeight - 4) // Account for borders and padding
	return m, nil
}

func (m model) handleSpecialKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	keyStr := msg.String()

	// Log all key presses
	if m.logger != nil {
		context := map[string]interface{}{
			"show_welcome": m.showWelcome,
			"show_output":  m.showOutput,
		}
		if !m.showWelcome {
			if selectedItem, ok := m.resourceList.SelectedItem().(resourceListItem); ok {
				context["selected_resource"] = selectedItem.title
			}
			if selectedItem, ok := m.operationList.SelectedItem().(resourceListItem); ok {
				context["selected_operation"] = selectedItem.title
			}
		}
		m.logger.LogKeyPress(keyStr, m.activeList, context)
	}

	if m.showWelcome {
		newModel, cmd := m.handleWelcomeKeys(msg)
		return newModel, cmd, true
	}

	// Handle filtering mode
	if m.filterMode {
		if newModel, cmd, handled := m.handleFilterKeys(msg); handled {
			return newModel, cmd, true
		}
	}

	// Handle scroll keys when output is visible
	if m.showOutput {
		if newModel, cmd, handled := m.handleScrollKeys(msg); handled {
			// Handle output scrolling if:
			// 1. Output console is selected (activeList == 3), OR
			// 2. Using page-based keys (not j/k/up/down) from any panel
			keyStr := msg.String()
			if m.activeList == 3 || keyStr == "page_up" || keyStr == "page_down" || keyStr == "ctrl+b" || keyStr == "ctrl+f" || keyStr == "home" || keyStr == "end" || keyStr == "g" || keyStr == "G" {
				return newModel, cmd, true
			}
		}
	}

	switch keyStr {
	case "ctrl+c", "q":
		m.quitting = true
		if m.logger != nil {
			m.logger.LogAction("quit_application", map[string]interface{}{
				"trigger": keyStr,
			})
		}
		return m, tea.Quit, true
	case "c":
		newModel, cmd := m.handleClearOutput()
		return newModel, cmd, true
	case "tab", "right", "left":
		oldView := getViewName(m.activeList)
		// Skip output console if no output is showing
		if m.showOutput {
			m.activeList = (m.activeList + 1) % 4
		} else {
			m.activeList = (m.activeList + 1) % 3
		}
		newView := getViewName(m.activeList)

		if m.logger != nil {
			m.logger.LogViewChange(oldView, newView, keyStr)
		}
		return m, nil, true
	case "enter":
		newModel, cmd := m.handleEnterKey()
		return newModel, cmd, true
	case "/":
		// Enable filter mode when in resource list
		m.filterMode = true
		m.filterText = ""
		if m.logger != nil {
			m.logger.LogAction("enter_filter_mode", map[string]interface{}{
				"active_list": m.activeList,
			})
		}
		return m, nil, true
	}
	return m, nil, false // Key not handled, let lists process it
}

func (m model) handleWelcomeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		if m.logger != nil {
			m.logger.LogAction("quit_from_welcome", map[string]interface{}{
				"trigger": msg.String(),
			})
		}
		return m, tea.Quit
	case "enter", " ":
		m.showWelcome = false
		if m.logger != nil {
			m.logger.LogStateChange("welcome_screen", "main_interface", map[string]interface{}{
				"trigger": msg.String(),
			})
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleClearOutput() (tea.Model, tea.Cmd) {
	if m.showOutput {
		if m.logger != nil {
			m.logger.LogAction("clear_output", map[string]interface{}{
				"previous_choice": m.choice,
			})
		}
		m.showOutput = false
		m.choice = ""
		m.commandOutput = ""
		m.outputScroll = 0
		return m, tea.WindowSize()
	}
	return m, nil
}

func (m model) handleEnterKey() (tea.Model, tea.Cmd) {
	switch m.activeList {
	case 0, 1: // Resource list or Operation list is active - allow execution from both
		resourceItem, resourceOk := m.resourceList.SelectedItem().(resourceListItem)
		operationItem, operationOk := m.operationList.SelectedItem().(resourceListItem)

		if m.logger != nil {
			m.logger.LogAction("enter_key", map[string]interface{}{
				"active_list":    m.activeList,
				"resource_item":  resourceItem.title,
				"operation_item": operationItem.title,
			})
		}

		// Execute if both resource and operation are selected
		if resourceOk && operationOk && resourceItem.resourceType != "separator" {
			if m.logger != nil {
				if m.activeList == 0 {
					m.logger.LogListSelection("resource", resourceItem.title, m.resourceList.Index())
				} else {
					m.logger.LogListSelection("operation", operationItem.title, m.operationList.Index())
				}
			}
			return m.executeAndAddToHistory(resourceItem, operationItem)
		}

		// If only resource is selected, update operations list and potentially switch to operations list
		if resourceOk && resourceItem.resourceType != "separator" {
			if m.logger != nil {
				m.logger.LogListSelection("resource", resourceItem.title, m.resourceList.Index())
			}
			// Update operations list when resource is selected
			m = m.updateOperationsList(resourceItem.title)

			// If we're on the resource list and there are operations, switch to operations list
			if m.activeList == 0 && len(m.operationList.Items()) > 0 {
				oldView := getViewName(m.activeList)
				m.activeList = 1
				newView := getViewName(m.activeList)

				if m.logger != nil {
					m.logger.LogViewChange(oldView, newView, "resource_selection")
				}
			}

			// If there's only one operation available, auto-execute it
			if len(m.operationList.Items()) == 1 {
				if firstOp := m.operationList.Items()[0]; firstOp != nil {
					if firstOpItem, ok := firstOp.(resourceListItem); ok {
						if m.logger != nil {
							m.logger.LogListSelection("operation", firstOpItem.title, 0)
							m.logger.LogAction("auto_select_single_operation", map[string]interface{}{
								"resource":  resourceItem.title,
								"operation": firstOpItem.title,
							})
						}
						return m.executeAndAddToHistory(resourceItem, firstOpItem)
					}
				}
			}
		}

		// If no operation selected but operations exist, use the first one (fallback for operation list only)
		if m.activeList == 1 && resourceOk && resourceItem.resourceType != "separator" && len(m.operationList.Items()) > 0 {
			if firstOp := m.operationList.Items()[0]; firstOp != nil {
				if firstOpItem, ok := firstOp.(resourceListItem); ok {
					if m.logger != nil {
						m.logger.LogListSelection("operation", firstOpItem.title, 0)
						m.logger.LogAction("auto_select_first_operation", map[string]interface{}{
							"resource":  resourceItem.title,
							"operation": firstOpItem.title,
						})
					}
					return m.executeAndAddToHistory(resourceItem, firstOpItem)
				}
			}
		}
	case 2: // Response history is active - no special action needed
		// History selection is handled in handleListUpdates
		if selectedItem, ok := m.responseHistory.SelectedItem().(historyItem); ok {
			if m.logger != nil {
				m.logger.LogListSelection("history", selectedItem.command, selectedItem.index)
			}
		}
	}
	return m, nil
}

func (m model) executeAndAddToHistory(resourceItem, operationItem resourceListItem) (tea.Model, tea.Cmd) {
	m.choice = resourceItem.title + " " + operationItem.title

	// Time the command execution
	startTime := time.Now()
	result, err := m.executeOperation(resourceItem.title, operationItem.title)
	duration := time.Since(startTime)

	// Log the command execution
	if m.logger != nil {
		m.logger.LogCommand(m.choice, &result, err, duration)
	}

	historyEntry := responseHistoryEntry{
		command:     m.choice,
		timestamp:   time.Now(),
		response:    result.output,
		method:      result.method,
		url:         result.url,
		requestBody: result.requestBody,
	}
	if err != nil {
		historyEntry.error = err.Error()
		m.commandOutput = fmt.Sprintf("Error executing command: %v", err)

		// Log the error details
		if m.logger != nil {
			m.logger.LogError("command_execution", err, map[string]interface{}{
				"resource":  resourceItem.title,
				"operation": operationItem.title,
				"duration":  duration.String(),
			})
		}
	} else {
		m.commandOutput = result.output

		// Log successful state change
		if m.logger != nil {
			m.logger.LogStateChange("command_idle", "command_result_displayed", map[string]interface{}{
				"command":  m.choice,
				"duration": duration.String(),
				"method":   result.method,
				"url":      result.url,
			})
		}
	}

	m.historyEntries = append([]responseHistoryEntry{historyEntry}, m.historyEntries...)
	m = m.updateResponseHistoryList()
	m.showOutput = true
	m.outputScroll = 0
	return m, tea.WindowSize()
}

func (m model) handleListUpdates(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.activeList {
	case 0:
		m.resourceList, cmd = m.resourceList.Update(msg)
		// Always update operations list when resource selection changes, even when filtering
		if selectedItem, ok := m.resourceList.SelectedItem().(resourceListItem); ok && selectedItem.resourceType != "separator" {
			m = m.updateOperationsList(selectedItem.title)
		}
	case 1:
		m.operationList, cmd = m.operationList.Update(msg)
	case 2:
		m.responseHistory, cmd = m.responseHistory.Update(msg)
		if selectedItem, ok := m.responseHistory.SelectedItem().(historyItem); ok {
			m.selectedResponse = selectedItem.index
			entry := m.historyEntries[selectedItem.index]

			// Build metadata section
			metadata := "Request Details:\n"
			metadata += fmt.Sprintf("  Method: %s\n", entry.method)
			metadata += fmt.Sprintf("  URL: %s\n", entry.url)
			if entry.requestBody != "" {
				metadata += fmt.Sprintf("  Request Body: %s\n", entry.requestBody)
			}
			metadata += fmt.Sprintf("  Timestamp: %s\n\n", entry.timestamp.Format("2006-01-02 15:04:05"))

			if entry.error != "" {
				metadata += fmt.Sprintf("Error: %s\n\n", entry.error)
			}

			metadata += "Response:\n"
			m.commandOutput = metadata + entry.response
			m.choice = entry.command
			m.showOutput = true
			m.outputScroll = 0
		}
	case 3:
		// Output console is selected - no list to update, but we can still process other keys
		// The output scrolling is handled in handleScrollKeys
		cmd = nil
	}
	return m, cmd
}

type historyItem struct {
	index     int
	command   string
	timestamp string
	status    string
}

func (h historyItem) FilterValue() string { return h.command }
func (h historyItem) Title() string {
	// Truncate command for narrow history panel
	if len(h.command) > 20 {
		return h.command[:17] + "..."
	}
	return h.command
}
func (h historyItem) Description() string {
	// Shorter description for narrow panel
	return fmt.Sprintf("%s %s", h.timestamp, h.status)
}

func (m model) updateResponseHistoryList() model {
	historyItems := make([]list.Item, 0, len(m.historyEntries))
	for i, entry := range m.historyEntries {
		status := "✓"
		if entry.error != "" {
			status = "✗"
		}
		historyItems = append(historyItems, historyItem{
			index:     i,
			command:   entry.command,
			timestamp: entry.timestamp.Format("15:04:05"),
			status:    status,
		})
	}
	m.responseHistory.SetItems(historyItems)
	return m
}

// getCommandPreview returns the current command preview string
func (m model) getCommandPreview() string {
	resourceItem, resourceOk := m.resourceList.SelectedItem().(resourceListItem)
	operationItem, operationOk := m.operationList.SelectedItem().(resourceListItem)

	if !resourceOk || resourceItem.resourceType == "separator" {
		return "stripe"
	}

	if !operationOk {
		return fmt.Sprintf("stripe %s", resourceItem.title)
	}

	return fmt.Sprintf("stripe %s %s", resourceItem.title, operationItem.title)
}

func (m model) handleFilterKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	keyStr := msg.String()

	switch keyStr {
	case "esc", "ctrl+c":
		// Exit filter mode
		m.filterMode = false
		m.filterText = ""
		m = m.resetFilteredLists()
		if m.logger != nil {
			m.logger.LogAction("exit_filter_mode", map[string]interface{}{
				"active_list": m.activeList,
			})
		}
		return m, nil, true
	case "enter":
		// Apply filter and exit filter mode
		m.filterMode = false
		if m.logger != nil {
			m.logger.LogAction("apply_filter", map[string]interface{}{
				"active_list": m.activeList,
				"filter_text": m.filterText,
			})
		}
		return m, nil, true
	case "backspace":
		// Remove last character
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m = m.applyFilter()
		}
		return m, nil, true
	default:
		// Add typed character to filter
		if len(keyStr) == 1 && keyStr >= " " && keyStr <= "~" {
			m.filterText += keyStr
			m = m.applyFilter()
		}
		return m, nil, true
	}
}

func (m model) handleScrollKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	keyStr := msg.String()

	switch keyStr {
	case "up", "k":
		if m.outputScroll > 0 {
			m.outputScroll--
		}
		return m, nil, true
	case "down", "j":
		// Calculate max scroll based on content
		outputLines := len(strings.Split(m.commandOutput, "\n"))
		visibleHeight := (m.height / 3) - 4
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		maxScroll := outputLines - visibleHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.outputScroll < maxScroll {
			m.outputScroll++
		}
		return m, nil, true
	case "page_up", "ctrl+b":
		visibleHeight := (m.height / 3) - 4
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		m.outputScroll -= visibleHeight
		if m.outputScroll < 0 {
			m.outputScroll = 0
		}
		return m, nil, true
	case "page_down", "ctrl+f":
		outputLines := len(strings.Split(m.commandOutput, "\n"))
		visibleHeight := (m.height / 3) - 4
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		maxScroll := outputLines - visibleHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		m.outputScroll += visibleHeight
		if m.outputScroll > maxScroll {
			m.outputScroll = maxScroll
		}
		return m, nil, true
	case "home", "g":
		m.outputScroll = 0
		return m, nil, true
	case "end", "G":
		outputLines := len(strings.Split(m.commandOutput, "\n"))
		visibleHeight := (m.height / 3) - 4
		if visibleHeight < 1 {
			visibleHeight = 1
		}
		maxScroll := outputLines - visibleHeight
		if maxScroll < 0 {
			maxScroll = 0
		}
		m.outputScroll = maxScroll
		return m, nil, true
	}

	return m, nil, false
}

func (m model) applyFilter() model {
	switch m.activeList {
	case 0: // Resource list
		filtered := make([]list.Item, 0)
		for _, rItem := range m.allResourceItems {
			if resourceItem, ok := rItem.(resourceListItem); ok {
				if strings.Contains(strings.ToLower(resourceItem.title), strings.ToLower(m.filterText)) ||
					strings.Contains(strings.ToLower(resourceItem.description), strings.ToLower(m.filterText)) {
					filtered = append(filtered, rItem)
				}
			}
		}
		m.resourceList.SetItems(filtered)

		// Auto-select the first filtered resource and update operations
		if len(filtered) > 0 {
			m.resourceList.Select(0) // Select first item
			if firstItem, ok := filtered[0].(resourceListItem); ok && firstItem.resourceType != "separator" {
				m = m.updateOperationsList(firstItem.title)
			}
		} else {
			// No filtered items, clear operations list
			m.operationList.SetItems([]list.Item{})
		}
	case 1: // Operation list
		filtered := make([]list.Item, 0)
		for _, rItem := range m.allOperationItems {
			if operationItem, ok := rItem.(resourceListItem); ok {
				if strings.Contains(strings.ToLower(operationItem.title), strings.ToLower(m.filterText)) ||
					strings.Contains(strings.ToLower(operationItem.description), strings.ToLower(m.filterText)) {
					filtered = append(filtered, rItem)
				}
			}
		}
		m.operationList.SetItems(filtered)
	}
	return m
}

func (m model) resetFilteredLists() model {
	switch m.activeList {
	case 0: // Resource list
		m.resourceList.SetItems(m.allResourceItems)
		// Update operations list for the currently selected resource after reset
		if selectedItem, ok := m.resourceList.SelectedItem().(resourceListItem); ok && selectedItem.resourceType != "separator" {
			m = m.updateOperationsList(selectedItem.title)
		}
	case 1: // Operation list
		m.operationList.SetItems(m.allOperationItems)
	}
	return m
}
