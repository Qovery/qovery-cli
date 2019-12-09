package cmd

import (
	"github.com/spf13/cobra"
)

var environmentStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Environment status",
	Long: `STATUS show an environment status. For example:

	qovery environment status`,

	Run: func(cmd *cobra.Command, args []string) {
		// TODO API call to list all instances
	},
}

func init() {
	environmentCmd.AddCommand(environmentStatusCmd)
}
