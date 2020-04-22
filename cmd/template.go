package cmd

import (
	"github.com/spf13/cobra"
)

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Perform templating actions",
	Long: `TEMPLATE performs templating actions. For example:

	qovery template`,
}

func init() {
	RootCmd.AddCommand(templateCmd)
}
