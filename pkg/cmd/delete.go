package cmd

import (
	"net/http"

	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/requests"
	"github.com/stripe/stripe-cli/pkg/validators"
)

type deleteCmd struct {
	reqs requests.Base
}

func newDeleteCmd() *deleteCmd {
	dc := &deleteCmd{}

	dc.reqs.Method = http.MethodDelete
	dc.reqs.Profile = &Config.Profile
	dc.reqs.Cmd = &cobra.Command{
		Use:   "delete <path>",
		Args:  validators.ExactArgs(1),
		Short: "Make a DELETE request to the Stripe API",
		Long: `Make DELETE requests to the Stripe API using your test mode key.

For a full list of supported paths, see the API reference:
https://stripe.com/docs/api

To delete a charge:

  $ stripe delete /customers/cus_FROPkgsHVRRspg`,
		RunE: dc.reqs.RunRequestsCmd,
	}

	dc.reqs.InitFlags()

	return dc
}
