package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var containerDomainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage container domains",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	containerCmd.AddCommand(containerDomainCmd)
}
