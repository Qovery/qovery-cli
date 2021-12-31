package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminLockByIdCmd = &cobra.Command{
		Use:   "lock",
		Short: "Lock a cluster with its Id",
		Run: func(cmd *cobra.Command, args []string) {
			lockClusterById()
		},
	}
)

func init() {
	adminLockByIdCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	orgaErr = adminLockByIdCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminLockByIdCmd)
}

func lockClusterById() {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.LockById(clusterId)
	}
}
