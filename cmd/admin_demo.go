package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminDemoCmd = &cobra.Command{
		Use:   "demo",
		Short: "get errors logs for the demo",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}
)

func init() {
	adminCmd.AddCommand(adminDemoCmd)
}
