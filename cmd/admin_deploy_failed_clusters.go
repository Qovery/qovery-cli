package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/spf13/cobra"
)

var (
	adminDeployFailedClustersCmd = &cobra.Command{
		Use:   "deploy-failed-clusters",
		Short: "Deploy all clusters that are in failed state",
		Run: func(cmd *cobra.Command, args []string) {
			deployFailedClusters()
		},
	}
)

func init() {
	adminCmd.AddCommand(adminDeployFailedClustersCmd)
}

func deployFailedClusters() {
	pkg.DeployFailedClusters()
}
