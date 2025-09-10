package cmd

import (
	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/cmd/tui"
	"github.com/stripe/stripe-cli/pkg/config"
	"github.com/stripe/stripe-cli/pkg/validators"
)

type tuiCmd struct {
	cmd      *cobra.Command
	livemode bool
}

func newTuiCmd(config *config.Config) *tuiCmd {
	tuiCmd := &tuiCmd{
		cmd: tui.NewTuiCmd(config.Profile),
	}

	tuiCmd.cmd.Flags().BoolVar(&tuiCmd.livemode, "live", false, "Make live requests (default: test)")
	tuiCmd.cmd.Args = validators.NoArgs

	return tuiCmd
}
