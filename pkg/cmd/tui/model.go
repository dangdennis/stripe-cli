package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

type item struct {
	title        string
	description  string
	resourceType string
}

func (i item) FilterValue() string { return i.title }
func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.description }

type animTickMsg struct{}

func doAnimTick() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return animTickMsg{}
	})
}

type model struct {
	resourceList  list.Model
	operationList list.Model
	choice        string
	commandOutput string
	quitting      bool
	activeList    int // 0 for resource list, 1 for operation list, 2 for output panel
	width         int
	height        int
	rootCmd       *cobra.Command
	showOutput    bool
	outputScroll  int
	showWelcome   bool
	animFrame     int
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
		if m.showWelcome {
			m.animFrame = (m.animFrame + 1) % 20 // Cycle animation frames
			return m, doAnimTick()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listWidth := (msg.Width - 4) / 2 // Split width between two lists with padding
		m.resourceList.SetWidth(listWidth)
		m.operationList.SetWidth(listWidth)

		// Calculate list height - leave space for output panel if shown
		outputPanelHeight := 0
		if m.showOutput {
			outputPanelHeight = msg.Height / 3 // Bottom third for output
		}
		listHeight := msg.Height - 6 - outputPanelHeight // Account for headers, status, and output panel

		m.resourceList.SetHeight(listHeight)
		m.operationList.SetHeight(listHeight)
		return m, nil

	case tea.KeyMsg:
		// Handle welcome screen navigation
		if m.showWelcome {
			switch msg.String() {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "enter", " ":
				m.showWelcome = false
				return m, nil
			}
			return m, nil
		}

		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "c":
			// Clear output panel
			if m.showOutput {
				m.showOutput = false
				m.choice = ""
				m.commandOutput = ""
				m.outputScroll = 0
				// Trigger window resize to adjust list heights
				return m, tea.WindowSize()
			}
			return m, nil

		case "tab", "right", "left":
			// Switch between lists/panels
			maxPanels := 2
			if m.showOutput {
				maxPanels = 3
			}
			m.activeList = (m.activeList + 1) % maxPanels
			return m, nil

		case "enter":
			if m.activeList == 0 {
				// Selected from resource list - update operations list
				i, ok := m.resourceList.SelectedItem().(item)
				if ok && i.resourceType != "separator" {
					m = m.updateOperationsList(i.title)
				}
			} else {
				// Selected from operation list - execute command
				i, ok := m.operationList.SelectedItem().(item)
				if ok {
					resourceItem, resourceOk := m.resourceList.SelectedItem().(item)
					if resourceOk {
						m.choice = resourceItem.title + " " + i.title
						output, err := m.executeCommand(resourceItem.title, i.title)
						if err != nil {
							m.commandOutput = fmt.Sprintf("Error executing command: %v", err)
						} else {
							m.commandOutput = output
						}
						m.showOutput = true
						m.outputScroll = 0 // Reset scroll for new output

						// Trigger window resize to adjust list heights
						return m, tea.WindowSize()
					} else {
						m.choice = i.title
						m.commandOutput = "Could not determine resource for command"
						m.showOutput = true
						m.outputScroll = 0 // Reset scroll for new output

						// Trigger window resize to adjust list heights
						return m, tea.WindowSize()
					}
				}
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.activeList == 0 {
		m.resourceList, cmd = m.resourceList.Update(msg)
		// Update operations when selection changes
		if selectedItem, ok := m.resourceList.SelectedItem().(item); ok && selectedItem.resourceType != "separator" {
			m = m.updateOperationsList(selectedItem.title)
		}
	} else if m.activeList == 1 {
		m.operationList, cmd = m.operationList.Update(msg)
	} else if m.activeList == 2 && m.showOutput {
		// Handle output panel scrolling
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up":
				if m.outputScroll > 0 {
					m.outputScroll--
				}
			case "down":
				// Calculate max scroll based on output length
				outputLines := strings.Count(m.commandOutput, "\n")
				outputPanelHeight := m.height / 3
				if outputPanelHeight < 5 {
					outputPanelHeight = 5
				}
				maxScroll := outputLines - (outputPanelHeight - 4) // Account for padding and borders
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.outputScroll < maxScroll {
					m.outputScroll++
				}
			}
		}
	}
	return m, cmd
}

func (m model) updateOperationsList(resourceName string) model {
	// Get operations for the selected resource
	operations := m.getResourceOperations(resourceName)

	// Create operation items
	operationItems := make([]list.Item, 0, len(operations))
	for _, op := range operations {
		operationItems = append(operationItems, item{
			title:       op,
			description: fmt.Sprintf("%s operation", op),
		})
	}

	// Update the operations list
	m.operationList.SetItems(operationItems)
	return m
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
