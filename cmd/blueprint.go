package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var blueprintServiceName string
var blueprintCmd = &cobra.Command{
	Use:   "blueprint",
	Short: "Manage blueprint services",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(blueprintCmd)
}
