package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/spf13/cobra"
)

var (
	adminDeleteOldInvalidCredentialsClustersCmd = &cobra.Command{
		Use:   "force-delete-old-invalid-credentials-clusters",
		Short: "Force delete clusters with invalid credentials with last updated date more thant n days",
		Run: func(cmd *cobra.Command, args []string) {
			deleteOldClustersWithInvalidCredentials()
		},
	}
)

func init() {
	adminDeleteOldInvalidCredentialsClustersCmd.Flags().IntVarP(&ageInDay, "cluster-last-update-in-days", "d", 30, "cluster last update in days")
	adminDeleteOldInvalidCredentialsClustersCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	adminCmd.AddCommand(adminDeleteOldInvalidCredentialsClustersCmd)
}

func deleteOldClustersWithInvalidCredentials() {
	pkg.DeleteOldClustersWithInvalidCredentials(ageInDay, dryRun)
}
