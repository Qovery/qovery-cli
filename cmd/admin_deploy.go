package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var(
	clusterId string
	orgaErr error
	dryRun bool
	adminDeployByIdCmd = &cobra.Command{
		Use: "deploy",
		Short: "Deploy  cluster with its Id",
		Run: func(cmd *cobra.Command, args []string){
		deployClusterById()
		},
	}
)

func init() {
	adminDeployByIdCmd.Flags().StringVarP(&clusterId,"cluster", "c","","Cluster's id")
	adminDeployByIdCmd.Flags().BoolVarP(&dryRun,"disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeployByIdCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminDeployByIdCmd)
}

func deployClusterById(){
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.DeployById(clusterId, dryRun)
	}
}
