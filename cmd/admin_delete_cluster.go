package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	adminDeleteClusterCmd = &cobra.Command{
		Use:   "delete-cluster",
		Short: "Delete cluster by id",
		Run: func(cmd *cobra.Command, args []string) {
			deleteClusterById()
		},
	}
)

func init() {
	adminDeleteClusterCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	adminDeleteClusterCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeleteClusterCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminDeleteClusterCmd)
}

func deleteClusterById() {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.DeleteClusterById(clusterId, dryRun)
	}
}
