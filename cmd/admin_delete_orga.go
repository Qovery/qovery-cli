package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/spf13/cobra"
)

var (
	adminDeleteOrgaCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete organization by the cluster's id it owns or by organization id",
		Run: func(cmd *cobra.Command, args []string) {
			deleteOrganizationByClusterId()
		},
	}
)

func init() {
	adminDeleteOrgaCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	adminDeleteOrgaCmd.Flags().StringVarP(&organizationId, "organization", "o", "", "Organization's id")
	adminDeleteOrgaCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	adminDeleteOrgaCmd.MarkFlagsMutuallyExclusive("cluster", "organization")
	adminDeleteOrgaCmd.MarkFlagsOneRequired("cluster", "organization")
	adminCmd.AddCommand(adminDeleteOrgaCmd)
}

func deleteOrganizationByClusterId() {
	if clusterId != "" {
		pkg.DeleteOrganizationByClusterId(clusterId, dryRun)
	} else {
		pkg.DeleteOrganizationByOrganizationId(organizationId, dryRun)
	}
}
