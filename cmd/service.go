package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage Qovery services",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
	},
}

func init() {
	rootCmd.AddCommand(serviceCmd)
}
