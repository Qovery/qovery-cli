package cmd

import (
	"github.com/spf13/cobra"
)

var routeCmd = &cobra.Command{
	Use:   "route",
	Short: "Perform route actions",
	Long: `ROUTE performs route actions on project environment. For example:

	qovery route`,
}

func init() {
	RootCmd.AddCommand(routeCmd)
}
