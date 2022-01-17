package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminDeleteOrgaCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete organization by the cluster's id it owns",
		Run: func(cmd *cobra.Command, args []string) {
			deleteOrganizationByClusterId()
		},
	}
)

func init() {
	adminDeleteOrgaCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	adminDeleteOrgaCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeleteOrgaCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminDeleteOrgaCmd)
}

func deleteOrganizationByClusterId() {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.DeleteOrganizationByClusterId(clusterId, dryRun)
	}
}
