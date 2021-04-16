package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var(
	clusterId string
	orgaErr error
	dryRun bool
	adminDeployCmd = &cobra.Command{
		Use: "deploy",
		Short: "Deploy organization's cluster with cluster's Id",
		Run: func(cmd *cobra.Command, args []string){
			deployClusterById()
		},
	}
)

func init() {
	adminDeployCmd.Flags().StringVarP(&clusterId,"cluster", "c","","Cluster's id")
	adminDeployCmd.Flags().BoolVarP(&dryRun,"disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeployCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminDeployCmd)
}

func deployClusterById(){
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		io.AdminDeploy(clusterId, dryRun)
	}
}