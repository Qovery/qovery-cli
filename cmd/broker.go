package cmd

import (
	"github.com/spf13/cobra"
)

var brokerCmd = &cobra.Command{
	Use:   "broker",
	Short: "Perform broker actions",
	Long: `BROKER performs route actions on project environment. For example:

	qovery broker`,
}

func init() {
	RootCmd.AddCommand(brokerCmd)
}
