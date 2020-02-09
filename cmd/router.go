package cmd

import (
	"github.com/spf13/cobra"
)

var routerCmd = &cobra.Command{
	Use:   "router",
	Short: "Perform router actions",
	Long: `ROUTER performs router actions on project environment. For example:

	qovery router`,
}

/*func init() {
	RootCmd.AddCommand(routerCmd)
}*/
