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
	list     list.Model
	choice   string
	quitting bool
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(item)
			if ok {
				m.choice = i.title
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.choice != "" {
		return quitTextStyle.Render(fmt.Sprintf("You selected: %s", m.choice))
	}
	if m.quitting {
		return quitTextStyle.Render("Thanks for using Stripe CLI TUI!")
	}
	return "\n" + m.list.View()
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

	// Create the list
	const defaultWidth = 40
	const listHeight = 40

	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	l.Title = "Stripe CLI Resources"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle

	m := model{list: l}

	// Show API key info at the top
	fmt.Printf("🔑 API Key: %s... (Live mode: %v)\n", apiKey[:min(7, len(apiKey))], tc.livemode)
	fmt.Println("📡 Use ↑/↓ to navigate, Enter to select, q to quit")
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
