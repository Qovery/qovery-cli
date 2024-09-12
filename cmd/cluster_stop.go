package cmd

import (
	"github.com/spf13/cobra"
	"os"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

var clusterStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a cluster",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		err = cluster.NewClusterService(client, &promptuifactory.PromptUiFactoryImpl{}).StopCluster(organizationName, clusterName, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	clusterCmd.AddCommand(clusterStopCmd)
	clusterStopCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	clusterStopCmd.Flags().StringVarP(&clusterName, "cluster", "n", "", "Cluster Name")
	clusterStopCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cluster status until it's ready or an error occurs")

	_ = clusterStopCmd.MarkFlagRequired("cluster")
}
