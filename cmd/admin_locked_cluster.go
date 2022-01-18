package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminLockedClustersCmd = &cobra.Command{
		Use:   "locked",
		Short: "List locked clusters",
		Run: func(cmd *cobra.Command, args []string) {
			lockedClusters()
		},
	}
)

func init() {
	adminCmd.AddCommand(adminLockedClustersCmd)
}

func lockedClusters() {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		pkg.LockedClusters()
	}
}
