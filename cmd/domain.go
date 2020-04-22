package cmd

import (
	"github.com/spf13/cobra"
)

var domainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Perform domain actions",
	Long: `DOMAIN performs actions on domains. For example:

	qovery domain`,
}

func init() {
	RootCmd.AddCommand(domainCmd)
}
