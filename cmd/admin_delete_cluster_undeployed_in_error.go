package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
)

var (
	adminDeleteClusterUnDeployedInErrorCmd = &cobra.Command{
		Use:   "delete-cluster-undeployed-in-error",
		Short: "Trigger deletion of all clusters not deployed once and that are in error",
		Run: func(cmd *cobra.Command, args []string) {
			deleteClusterUnDeployedInError()
		},
	}
)

func init() {
	adminCmd.AddCommand(adminDeleteClusterUnDeployedInErrorCmd)
}

func deleteClusterUnDeployedInError() {
	pkg.DeleteClusterUnDeployedInError()
}
