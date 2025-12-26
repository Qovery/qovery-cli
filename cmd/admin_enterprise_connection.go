package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminEnterpriseConnectionCmd = &cobra.Command{
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
	enterpriseConnectionName           string
	enterpriseConnectionOrganizationId string
)

func init() {
	adminCmd.AddCommand(adminEnterpriseConnectionCmd)
}

type EnterpriseConnection struct {
	OrganizationID string `json:"organization_id"`
	ConnectionName string `json:"connection_name"`
	DefaultRole    string `json:"default_role"`
}
