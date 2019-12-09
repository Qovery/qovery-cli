package cmd

import (
	"github.com/spf13/cobra"
)

var applicationCmd = &cobra.Command{
	Use:   "application",
	Short: "Perform application actions",
	Long: `APPLICATION performs application actions on project environment. For example:

	qovery application`,
}

func init() {
	RootCmd.AddCommand(applicationCmd)
}
