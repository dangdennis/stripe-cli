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

	// Create response history panel
	historyView := m.responseHistory.View()
	historyBorderColor := lipgloss.Color("240")
	if m.activeList == 2 {
		historyBorderColor = lipgloss.Color("170")
	}

	historyBorder := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(historyBorderColor).
		Width(m.width - 4)

	historyPanel := historyBorder.Render(historyView)

	// Create the main layout with three sections
	mainLayout := lipgloss.JoinVertical(lipgloss.Left, topPanels, historyPanel)

	// If no output to show, return layout without output panel
	if !m.showOutput {
		return mainLayout
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

	// Create output panel (no longer needs highlighting as it's not selectable)
	outputBorderColor := lipgloss.Color("240")

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

	// Join main layout and output panel vertically
	return lipgloss.JoinVertical(lipgloss.Left, mainLayout, outputPanel)
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
