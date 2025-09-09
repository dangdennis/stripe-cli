package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/validators"
)

type tuiCmd struct {
	cmd      *cobra.Command
	livemode bool
}

func newTuiCmd() *tuiCmd {
	tc := &tuiCmd{}
	tc.cmd = &cobra.Command{
		Use:   "tui",
		Args:  validators.NoArgs,
		Short: "Start the Stripe CLI TUI (Terminal User Interface)",
		Long:  `Launch an interactive terminal user interface for the Stripe CLI using Charm's Bubble Tea framework.`,
		RunE:  tc.runTui,
	}

	tc.cmd.Flags().BoolVar(&tc.livemode, "livemode", false, "Use live mode API keys")
	return tc
}

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

func (m model) View() string {
	if m.showWelcome {
		return m.welcomeView()
	}

	if m.quitting {
		return quitTextStyle.Render("Thanks for using Stripe CLI TUI!")
	}

	// Top panels - side-by-side layout
	resourceView := m.resourceList.View()
	operationView := m.operationList.View()

	// Add borders and highlighting for active list
	resourceBorder := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width((m.width - 4) / 2)
	operationBorder := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width((m.width - 4) / 2)

	switch m.activeList {
	case 0:
		resourceBorder = resourceBorder.BorderForeground(lipgloss.Color("170"))
	case 1:
		operationBorder = operationBorder.BorderForeground(lipgloss.Color("170"))
	}

	resourcePanel := resourceBorder.Render(resourceView)
	operationPanel := operationBorder.Render(operationView)

	// Join top panels horizontally
	topPanels := lipgloss.JoinHorizontal(lipgloss.Top, resourcePanel, operationPanel)

	// If no output to show, return just the top panels
	if !m.showOutput {
		return topPanels
	}

	// Create bottom output panel
	outputTitle := ""
	if m.choice != "" {
		outputTitle = fmt.Sprintf("Command Output: stripe %s", m.choice)
	}

	outputContent := m.commandOutput
	if outputContent == "" {
		outputContent = "No output"
	}

	// Handle scrolling for output content
	outputLines := strings.Split(outputContent, "\n")
	outputHeight := m.height / 3
	if outputHeight < 5 {
		outputHeight = 5
	}

	// Calculate visible content based on scroll position
	visibleHeight := outputHeight - 4 // Account for title, padding and borders
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	startLine := m.outputScroll
	endLine := startLine + visibleHeight
	if endLine > len(outputLines) {
		endLine = len(outputLines)
	}
	if startLine >= len(outputLines) {
		startLine = len(outputLines) - 1
		if startLine < 0 {
			startLine = 0
		}
	}

	var visibleContent string
	if len(outputLines) > 0 && startLine < len(outputLines) {
		visibleContent = strings.Join(outputLines[startLine:endLine], "\n")
	}

	// Create output panel with highlighting if active
	outputBorderColor := lipgloss.Color("240")
	if m.activeList == 2 {
		outputBorderColor = lipgloss.Color("170")
	}

	outputBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(outputBorderColor).
		Width(m.width - 4).
		Height(outputHeight - 2). // Account for border
		Padding(1)

	scrollInfo := ""
	if len(outputLines) > visibleHeight {
		scrollInfo = fmt.Sprintf(" [%d-%d/%d lines]", startLine+1, endLine, len(outputLines))
	}

	outputPanel := outputBorder.Render(
		lipgloss.NewStyle().
			Bold(true).
			Render(outputTitle+scrollInfo) + "\n\n" + visibleContent,
	)

	// Join top panels and bottom output panel vertically
	return lipgloss.JoinVertical(lipgloss.Left, topPanels, outputPanel)
}

func (m model) welcomeView() string {
	// ASCII art for STRIPE logo
	logo := []string{
		"  ██████ ████████ ██████  ██ ██████  ███████ ",
		" ██         ██    ██   ██ ██ ██   ██ ██      ",
		" ██████     ██    ██████  ██ ██████  █████   ",
		"      ██    ██    ██   ██ ██ ██      ██      ",
		" ██████     ██    ██   ██ ██ ██      ███████ ",
	}

	// Create animated gradient colors
	colors := []string{"#ff6b6b", "#4ecdc4", "#45b7d1", "#96ceb4", "#feca57", "#ff9ff3", "#54a0ff", "#5f27cd"}
	baseColor := colors[m.animFrame%len(colors)]

	// Create animated logo with changing colors
	var styledLogo []string
	for i, line := range logo {
		// Use different colors for each line with animation offset
		colorIndex := (m.animFrame + i*2) % len(colors)
		lineStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(colors[colorIndex])).
			Bold(true)
		styledLogo = append(styledLogo, lineStyle.Render(line))
	}

	logoBlock := lipgloss.JoinVertical(lipgloss.Center, styledLogo...)

	// Add sparkle animation around the logo
	sparkles := ""
	sparklePos := m.animFrame % 8
	switch sparklePos {
	case 0, 4:
		sparkles = "✨ ⭐ ✨"
	case 1, 5:
		sparkles = "⭐ ✨ ⭐"
	case 2, 6:
		sparkles = "💫 ✨ 💫"
	case 3, 7:
		sparkles = "✨ 💫 ✨"
	}

	sparkleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(baseColor)).
		Bold(true)

	// Welcome text and instructions
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color(baseColor)).
		Bold(true).
		Padding(0, 2).
		Margin(1, 0)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true).
		Margin(1, 0)

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(baseColor)).
		Padding(1, 2).
		Margin(1, 2)

	// Panel descriptions
	panelInfo := panelStyle.Render(strings.Join([]string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#4ecdc4")).Bold(true).Render("📋 Resources Panel (Left)"),
		"• Browse Stripe API resources",
		"• Customers, Charges, Payment Intents, etc.",
		"• Navigate with ↑/↓ arrows",
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9ff3")).Bold(true).Render("⚡ Operations Panel (Right)"),
		"• Available operations for selected resource",
		"• Create, List, Update, Delete, etc.",
		"• Execute with Enter key",
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#feca57")).Bold(true).Render("📊 Output Panel (Bottom)"),
		"• Live command results with JSON formatting",
		"• Scrollable with ↑/↓ when focused",
		"• Switch panels with ←/→ or Tab",
	}, "\n"))

	// Instructions
	instructionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#333333")).
		Bold(true).
		Padding(0, 2).
		Margin(2, 0)

	instructions := instructionStyle.Render("Press ENTER or SPACE to continue • Press Q to quit")

	// Combine all elements
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		sparkleStyle.Render(sparkles),
		logoBlock,
		sparkleStyle.Render(sparkles),
		titleStyle.Render("Welcome to the Stripe TUI"),
		subtitleStyle.Render("The ultimate terminal interface for the Stripe API"),
		panelInfo,
		instructions,
	)

	// Center everything in the terminal
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}

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

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

func (tc *tuiCmd) runTui(cmd *cobra.Command, args []string) error {
	// Get the API key from the user's config
	apiKey, err := Config.Profile.GetAPIKey(tc.livemode)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}

	// Create list items for V1 resources using the same logic as resources command
	items := []list.Item{}

	// Get V1 resources from root command
	rootCmd := cmd.Root()
	for _, subCmd := range rootCmd.Commands() {
		if subCmd.Hidden {
			continue
		}

		annotation := rootCmd.Annotations[subCmd.Name()]
		if annotation == "resource" || annotation == "namespace" {
			description := subCmd.Short
			if description == "" {
				description = "Stripe resource"
			}
			items = append(items, item{
				title:        subCmd.Name(),
				description:  description,
				resourceType: "v1",
			})
		}
	}

	// Add separator for V2 resources
	items = append(items, item{
		title:        "--- V2 Resources ---",
		description:  "Below are V2 API resources",
		resourceType: "separator",
	})

	// Get V2 resources
	for _, subCmd := range rootCmd.Commands() {
		if subCmd.Name() == "v2" {
			for _, v2SubCmd := range subCmd.Commands() {
				if v2SubCmd.Hidden {
					continue
				}
				description := v2SubCmd.Short
				if description == "" {
					description = "Stripe V2 resource"
				}
				items = append(items, item{
					title:        "v2 " + v2SubCmd.Name(),
					description:  description,
					resourceType: "v2",
				})
			}
			break
		}
	}

	// Create the resource list
	const defaultWidth = 40
	const listHeight = 20

	resourceList := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	resourceList.Title = "Resources"
	resourceList.SetShowStatusBar(false)
	resourceList.SetFilteringEnabled(false)
	resourceList.Styles.Title = titleStyle
	resourceList.Styles.PaginationStyle = paginationStyle
	resourceList.Styles.HelpStyle = helpStyle

	// Create the operations list (initially empty)
	operationList := list.New([]list.Item{}, itemDelegate{}, defaultWidth, listHeight)
	operationList.Title = "Operations"
	operationList.SetShowStatusBar(false)
	operationList.SetFilteringEnabled(false)
	operationList.Styles.Title = titleStyle
	operationList.Styles.PaginationStyle = paginationStyle
	operationList.Styles.HelpStyle = helpStyle

	m := model{
		resourceList:  resourceList,
		operationList: operationList,
		activeList:    0, // Start with resource list active
		rootCmd:       cmd.Root(),
		showWelcome:   true, // Show welcome screen initially
		animFrame:     0,
	}

	// Initialize operations for the first resource
	if len(items) > 0 {
		if firstItem, ok := items[0].(item); ok && firstItem.resourceType != "separator" {
			m = m.updateOperationsList(firstItem.title)
		}
	}

	// Show API key info at the top
	fmt.Printf("🔑 API Key: %s... (Live mode: %v)\n", apiKey[:min(7, len(apiKey))], tc.livemode)
	fmt.Println("📡 Use ↑/↓ to navigate/scroll, ←/→/Tab to switch panels, Enter to execute, c to clear output, q to quit")
	fmt.Println()

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	var output string
	if i.resourceType == "separator" {
		output = selectedItemStyle.Render("• " + strings.Repeat("─", len(i.title)))
	} else {
		str := fmt.Sprintf("%d. %s", index+1, i.title)

		fn := itemStyle.Render
		if index == m.Index() {
			fn = func(s ...string) string {
				return selectedItemStyle.Render("> " + strings.Join(s, " "))
			}
		}

		output = fn(str)
	}

	fmt.Fprint(w, output)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
