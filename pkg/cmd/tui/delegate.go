package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type listItemDelegate struct{}

func (d listItemDelegate) Height() int                             { return 1 }
func (d listItemDelegate) Spacing() int                            { return 0 }
func (d listItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d listItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(resourceListItem)
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
