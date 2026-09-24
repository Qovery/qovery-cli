package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	enterpriseConnectionGroupMappingsCmd = &cobra.Command{
		Use:   "group-mappings",
		Short: "Manage enterprise connection group mappings",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			if len(args) == 0 {
				_ = cmd.Help()
				os.Exit(0)
			}
		},
	}
	qoveryRole    string
	idpGroupNames string
)

func init() {
	enterpriseConnectionCmd.AddCommand(enterpriseConnectionGroupMappingsCmd)
}
