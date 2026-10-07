package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var blueprintCmd = &cobra.Command{
	Use:   "blueprint",
	Short: "Manage blueprints",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.AddCommand(blueprintCmd)
}
