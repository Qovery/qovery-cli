package cmd

import (
	"github.com/spf13/cobra"
)

var environmentCmd = &cobra.Command{
	Use:   "environment",
	Aliases: []string{"env"},
	Short: "Perform environment actions",
	Long: `ENVIRONMENT performs actions on project environment. For example:

	qovery environment`,
}

func init() {
	RootCmd.AddCommand(environmentCmd)
}
