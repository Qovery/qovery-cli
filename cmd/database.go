package cmd

import (
	"github.com/spf13/cobra"
)

var databaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Perform database actions",
	Long: `DATABASE performs route actions on project environment. For example:

	qovery database`,
}

func init() {
	RootCmd.AddCommand(databaseCmd)
}
