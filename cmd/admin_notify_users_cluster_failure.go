package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var (
	adminNotifyUsersClusterFailureCmd = &cobra.Command{
		Use:   "notify-users-cluster-failure",
		Short: "Notify users of a cluster failure",
		Long: `Notify users by email of a cluster having FAILED status.
- (Default) With --cluster-id, only admins of the cluster with the given id will be notified.
- Without --cluster-id, admins of all clusters with FAILED status will be notified.
`,
		Run: func(cmd *cobra.Command, args []string) {
			notifyUsersClusterFailure()
		},
	}
)

func init() {
	adminNotifyUsersClusterFailureCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	adminCmd.AddCommand(adminNotifyUsersClusterFailureCmd)
}

func notifyUsersClusterFailure() {
	utils.CheckAdminUrl()

	err := pkg.NotifyUsersClusterFailure(&clusterId)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}
