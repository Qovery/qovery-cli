package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var (
	detachDatabaseId     string
	detachDatabaseReason string

	adminDetachDatabaseCmd = &cobra.Command{
		Use:   "detach-database",
		Short: "Remove a managed database from Qovery only: no deployment, nothing touched on the cloud account",
		Long: `Remove a legacy managed database service and its built-in variables from Qovery, without any
action on the customer's cloud account: the instance, its snapshots and its network/IAM resources stay.

Runs a dry run by default. Refused when the database is a container or blueprint-backed one, when its
environment is deploying or has a queued request, or when an alias or override relies on its variables.`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if err := pkg.DetachDatabase(detachDatabaseId, detachDatabaseReason, dryRun); err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	adminDetachDatabaseCmd.Flags().StringVarP(&detachDatabaseId, "database", "d", "", "Database's id")
	adminDetachDatabaseCmd.Flags().StringVarP(&detachDatabaseReason, "reason", "r", "", "Why (support ticket, customer request): recorded in the customer's audit log")
	adminDetachDatabaseCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	_ = adminDetachDatabaseCmd.MarkFlagRequired("database")
	_ = adminDetachDatabaseCmd.MarkFlagRequired("reason")
	adminCmd.AddCommand(adminDetachDatabaseCmd)
}
