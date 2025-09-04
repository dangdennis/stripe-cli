package cmd

import (
	"fmt"
	"io"
	"strings"

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

type model struct {
	resourceList  list.Model
	operationList list.Model
	choice        string
	quitting      bool
	activeList    int // 0 for resource list, 1 for operation list
	width         int
	height        int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listWidth := (msg.Width - 4) / 2 // Split width between two lists with padding
		m.resourceList.SetWidth(listWidth)
		m.operationList.SetWidth(listWidth)
		m.resourceList.SetHeight(msg.Height - 6) // Account for headers and status
		m.operationList.SetHeight(msg.Height - 6)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "tab":
			// Switch between lists
			m.activeList = (m.activeList + 1) % 2
			return m, nil

		case "enter":
			if m.activeList == 0 {
				// Selected from resource list - update operations list
				i, ok := m.resourceList.SelectedItem().(item)
				if ok && i.resourceType != "separator" {
					m = m.updateOperationsList(i.title)
				}
			} else {
				// Selected from operation list - execute choice
				i, ok := m.operationList.SelectedItem().(item)
				if ok {
					resourceItem, resourceOk := m.resourceList.SelectedItem().(item)
					if resourceOk {
						m.choice = resourceItem.title + " " + i.title
					} else {
						m.choice = i.title
					}
				}
				return m, tea.Quit
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
	} else {
		m.operationList, cmd = m.operationList.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("You selected: %s", m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Thanks for using Stripe CLI TUI!")
	}

	// Side-by-side layout
	resourceView := m.resourceList.View()
	operationView := m.operationList.View()

	// Add borders and highlighting for active list
	resourceBorder := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width((m.width - 4) / 2)
	operationBorder := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Width((m.width - 4) / 2)

	if m.activeList == 0 {
		resourceBorder = resourceBorder.BorderForeground(lipgloss.Color("170"))
	} else {
		operationBorder = operationBorder.BorderForeground(lipgloss.Color("170"))
	}

	resourcePanel := resourceBorder.Render(resourceView)
	operationPanel := operationBorder.Render(operationView)

	// Join panels horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, resourcePanel, operationPanel)
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
	}

	// Initialize operations for the first resource
	if len(items) > 0 {
		if firstItem, ok := items[0].(item); ok && firstItem.resourceType != "separator" {
			m = m.updateOperationsList(firstItem.title)
		}
	}

	// Show API key info at the top
	fmt.Printf("🔑 API Key: %s... (Live mode: %v)\n", apiKey[:min(7, len(apiKey))], tc.livemode)
	fmt.Println("📡 Use ↑/↓ to navigate, Tab to switch lists, Enter to select, q to quit")
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
