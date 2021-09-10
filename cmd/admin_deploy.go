package cmd

import (
	"github.com/qovery/qovery-cli/io"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var(
	clusterId string
	orgaErr error
	dryRun bool
	adminDeployByIdCmd = &cobra.Command{
		Use: "deploy",
		Short: "Deploy organization's cluster with cluster's Id",
		Run: func(cmd *cobra.Command, args []string){
		deployClusterById()
		},
	}

	adminDeployAllCmd = &cobra.Command{
		Use: "deploy",
		Short: "Deploy organization's cluster with cluster's Id",
		Run: func(cmd *cobra.Command, args []string){
			deployAllClusters()
		},
	}
)

func init() {
	adminDeployByIdCmd.Flags().StringVarP(&clusterId,"cluster", "c","","Cluster's id")
	adminDeployByIdCmd.Flags().BoolVarP(&dryRun,"disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeployByIdCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminDeployByIdCmd)

	adminDeployAllCmd.Flags().BoolVarP(&dryRun,"disable-dry-run", "y", false, "Disable dry run mode")
	adminCmd.AddCommand(adminDeployAllCmd)
}

func deployClusterById(){
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		io.DeployById(clusterId, dryRun)
	}
}

func deployAllClusters() {
	io.DeployAll(dryRun)
}
