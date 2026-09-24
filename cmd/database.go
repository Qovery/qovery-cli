package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var databaseName string
var databaseNames string
var showCredentials bool
var databaseCmd = &cobra.Command{
	Use:   "database",
	Short: "Manage databases",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(databaseCmd)
}
