package cmd

import (
	"github.com/stripe/stripe-cli/pkg/cmd/tui"
	"github.com/stripe/stripe-cli/pkg/config"
)

func newTuiCmd(config *config.Config) *tui.TUICmd {
	return tui.NewTuiCmd(config.Profile)
}
