package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	enterpriseConnectionCmd = &cobra.Command{
		Use:   "enterprise-connection",
		Short: "Manage enterprise connections",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}
	connectionName   string
	defaultRole      string
	enforceGroupSync bool
)

func init() {
	rootCmd.AddCommand(enterpriseConnectionCmd)
}
