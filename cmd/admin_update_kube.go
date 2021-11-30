package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminUpdateByIdCmd = &cobra.Command{
		Use:   "update",
		Short: "Update cluster with its Id to a specific version",
		Run: func(cmd *cobra.Command, args []string) {
			updateClusterById()
		},
	}
)

func init() {
	adminUpdateByIdCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	adminUpdateByIdCmd.Flags().StringVarP(&version, "version", "v", "", "Targeted version")
	adminUpdateByIdCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminUpdateByIdCmd.MarkFlagRequired("cluster")
	versionErr = adminUpdateByIdCmd.MarkFlagRequired("version")
	adminCmd.AddCommand(adminUpdateByIdCmd)
}

func updateClusterById() {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.UpdateById(clusterId, dryRun, version)
	}
}
