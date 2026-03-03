package cmd

import (
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage container registries",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	containerCmd.AddCommand(containerRegistryCmd)
}
