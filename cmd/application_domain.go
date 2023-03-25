package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var applicationDomainCmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage application domains",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	applicationCmd.AddCommand(applicationDomainCmd)
}
