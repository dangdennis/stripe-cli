package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/config"
)

// NewTuiCmd creates a new TUI command
func NewTuiCmd(config config.Profile) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Start the Stripe TUI",
		Long:  `Launch an interactive terminal user interface for the Stripe CLI`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTui(cmd, config)
		},
	}

	return cmd
}

func runTui(cmd *cobra.Command, profile config.Profile) error {
	apiKey := profile.APIKey

	// Create list items for V1 resources using the same logic as resources command
	items := []list.Item{}

	// Get V1 resources from root command
	rootCmd := cmd.Root()
	for _, subCmd := range rootCmd.Commands() {
		if subCmd.Hidden {
			continue
		}

		annotation := rootCmd.Annotations[subCmd.Name()]
		if annotation == "resource" {
			description := subCmd.Short
			if description == "" {
				description = "Stripe resource"
			}
			items = append(items, item{
				title:        subCmd.Name(),
				description:  description,
				resourceType: "v1",
			})
		} else if annotation == "namespace" {
			// For namespace commands, add their resource subcommands
			for _, nsSubCmd := range subCmd.Commands() {
				if nsSubCmd.Hidden {
					continue
				}
				nsAnnotation := subCmd.Annotations[nsSubCmd.Name()]
				if nsAnnotation == "resource" {
					description := nsSubCmd.Short
					if description == "" {
						description = fmt.Sprintf("%s resource", subCmd.Name())
					}
					items = append(items, item{
						title:        subCmd.Name() + " " + nsSubCmd.Name(),
						description:  description,
						resourceType: "v1",
					})
				}
			}
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

				v2Annotation := subCmd.Annotations[v2SubCmd.Name()]
				if v2Annotation == "resource" {
					description := v2SubCmd.Short
					if description == "" {
						description = "Stripe V2 resource"
					}
					items = append(items, item{
						title:        "v2 " + v2SubCmd.Name(),
						description:  description,
						resourceType: "v2",
					})
				} else if v2Annotation == "namespace" {
					// For V2 namespace commands, add their resource subcommands
					for _, v2NsSubCmd := range v2SubCmd.Commands() {
						if v2NsSubCmd.Hidden {
							continue
						}
						v2NsAnnotation := v2SubCmd.Annotations[v2NsSubCmd.Name()]
						if v2NsAnnotation == "resource" {
							description := v2NsSubCmd.Short
							if description == "" {
								description = fmt.Sprintf("V2 %s resource", v2SubCmd.Name())
							}
							items = append(items, item{
								title:        "v2 " + v2SubCmd.Name() + " " + v2NsSubCmd.Name(),
								description:  description,
								resourceType: "v2",
							})
						}
					}
				}
			}
			break
		}
	}

	// Create the resource list
	const defaultWidth = 40
	const listHeight = 20

	resourceList := list.New(items, listItemDelegate{}, defaultWidth, listHeight)
	resourceList.Title = "Resources"
	resourceList.SetShowStatusBar(false)
	resourceList.SetFilteringEnabled(false)
	resourceList.Styles.Title = titleStyle
	resourceList.Styles.PaginationStyle = paginationStyle
	resourceList.Styles.HelpStyle = helpStyle

	// Create the operations list (initially empty)
	operationList := list.New([]list.Item{}, listItemDelegate{}, defaultWidth, listHeight)
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
	fmt.Printf("🔑 API Key: %s... (Live mode: %v)\n", apiKey[:min(7, len(apiKey))], profile.APIKey)
	fmt.Println("📡 Use ↑/↓ to navigate/scroll, ←/→/Tab to switch panels, Enter to execute, c to clear output, q to quit")
	fmt.Println()

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
