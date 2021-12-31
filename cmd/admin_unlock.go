package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminUnlockByIdCmd = &cobra.Command{
		Use:   "unlock",
		Short: "Unlock a cluster with its Id",
		Run: func(cmd *cobra.Command, args []string) {
			unlockClusterById()
		},
	}
)

func init() {
	adminUnlockByIdCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	orgaErr = adminUnlockByIdCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminUnlockByIdCmd)
}

func unlockClusterById() {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.UnockById(clusterId)
	}
}
