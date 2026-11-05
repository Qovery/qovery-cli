package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var (
	requireExtendedCredentialsProduction bool
	adminRequireExtendedCredentialsCmd   = &cobra.Command{
		Use:   "requireExtendedCredentials",
		Short: "Require extended credentials to deploy clusters",
		Run: func(cmd *cobra.Command, args []string) {
			requireExtendedCredentials()
		},
	}
)

func init() {
	adminRequireExtendedCredentialsCmd.Flags().BoolVar(&requireExtendedCredentialsProduction, "production", false, "Require extended credentials for production clusters")
	adminCmd.AddCommand(adminRequireExtendedCredentialsCmd)
}

func requireExtendedCredentials() {
	err := pkg.RequireExtendedCredentials(requireExtendedCredentialsProduction)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
}
