package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = lipgloss.NewStyle().PaddingLeft(4)
	helpStyle         = lipgloss.NewStyle().PaddingLeft(4).PaddingBottom(1)
	quitTextStyle     = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

func (m model) View() string {
	if m.showWelcome {
		return m.welcomeView()
	}

	if m.quitting {
		return quitTextStyle.Render("Thanks for using Stripe CLI TUI!")
	}

	// Command preview line
	commandPreview := m.getCommandPreview()
	previewStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("170")).
		Background(lipgloss.Color("236")).
		Bold(true).
		Width(m.width).
		Padding(0, 2).
		Align(lipgloss.Left)

	previewLine := previewStyle.Render(commandPreview)

	// Top panels - side-by-side layout
	resourceView := m.resourceList.View()
	operationView := m.operationList.View()

	// Add borders and highlighting for active list
	resourceBorder := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width((m.width - 4) / 2)
	operationBorder := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width((m.width - 4) / 2)

	// Add filter indicator to titles
	resourceTitle := "Resources"
	operationTitle := "Operations"

	if m.filterMode {
		switch m.activeList {
		case 0:
			resourceTitle = fmt.Sprintf("Resources (Filter: %s)", m.filterText)
		case 1:
			operationTitle = fmt.Sprintf("Operations (Filter: %s)", m.filterText)
		}
	}

	switch m.activeList {
	case 0:
		resourceBorder = resourceBorder.BorderForeground(lipgloss.Color("170"))
		if m.filterMode {
			resourceBorder = resourceBorder.BorderForeground(lipgloss.Color("226")) // Yellow when filtering
		}
	case 1:
		operationBorder = operationBorder.BorderForeground(lipgloss.Color("170"))
		if m.filterMode {
			operationBorder = operationBorder.BorderForeground(lipgloss.Color("226")) // Yellow when filtering
		}
	}

	// Update list titles before rendering
	m.resourceList.Title = resourceTitle
	m.operationList.Title = operationTitle

	resourcePanel := resourceBorder.Render(resourceView)
	operationPanel := operationBorder.Render(operationView)

	// Join top panels horizontally
	topPanels := lipgloss.JoinHorizontal(lipgloss.Top, resourcePanel, operationPanel)

	// Create bottom area layout (history + output)
	bottomHeight := m.height / 3
	if bottomHeight < 5 {
		bottomHeight = 5
	}

	// History panel dimensions (narrow left side)
	historyWidth := 25 // Fixed narrow width for history
	if historyWidth > m.width/4 {
		historyWidth = m.width / 4 // But not more than 1/4 of screen
	}

	// Create response history panel (narrow left side)
	historyView := m.responseHistory.View()
	historyBorderColor := lipgloss.Color("240")
	if m.activeList == 2 {
		historyBorderColor = lipgloss.Color("170")
	}

	historyBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(historyBorderColor).
		Width(historyWidth).
		Height(bottomHeight - 2) // Account for border

	historyPanel := historyBorder.Render(historyView)

	// If no output to show, just show history panel alone
	if !m.showOutput {
		// Add help text
		helpText := "Tab: Switch panels • /: Filter • Enter: Execute • c: Clear • q: Quit"
		if m.filterMode {
			helpText = "Type to filter • Enter: Apply • Esc: Cancel"
		}

		helpStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 2)

		helpLine := helpStyle.Render(helpText)

		// Create the main layout with preview line, top panels, and just history
		mainLayout := lipgloss.JoinVertical(lipgloss.Left, previewLine, topPanels, historyPanel, helpLine)
		return mainLayout
	}

	// Create output panel (right side, taking remaining width)
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

	// Calculate visible content based on scroll position
	visibleHeight := bottomHeight - 4 // Account for title, padding and borders
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

	// Create output panel (taking remaining width)
	outputWidth := m.width - historyWidth - 6 // Account for borders and spacing
	outputBorderColor := lipgloss.Color("240")

	outputBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(outputBorderColor).
		Width(outputWidth).
		Height(bottomHeight - 2). // Account for border
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

	// Join history and output panels horizontally
	bottomPanel := lipgloss.JoinHorizontal(lipgloss.Top, historyPanel, outputPanel)

	// Add help text
	helpText := "Tab: Switch panels • /: Filter • ↑↓/jk: Navigate lists • PgUp/PgDn: Scroll output • Home/End: Output top/bottom • c: Clear • q: Quit"
	if m.filterMode {
		helpText = "Type to filter • Enter: Apply • Esc: Cancel"
	}

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 2)

	helpLine := helpStyle.Render(helpText)

	// Join main layout with bottom panel and help vertically
	mainLayout := lipgloss.JoinVertical(lipgloss.Left, previewLine, topPanels, bottomPanel, helpLine)
	return mainLayout
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
		instructions,
	)

	// Center everything in the terminal
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
}
