package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var environmentCmd = &cobra.Command{
	Use:   "environment",
	Short: "Manage Qovery environments",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
	},
}

func init() {
	rootCmd.AddCommand(environmentCmd)
}
