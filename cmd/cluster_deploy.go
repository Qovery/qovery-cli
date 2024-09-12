package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
)

var clusterDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a cluster",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		err = cluster.NewClusterService(client, &promptuifactory.PromptUiFactoryImpl{}).DeployCluster(organizationName, clusterName, watchFlag)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	clusterCmd.AddCommand(clusterDeployCmd)
	clusterDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	clusterDeployCmd.Flags().StringVarP(&clusterName, "cluster", "n", "", "Cluster Name")
	clusterDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cluster status until it's ready or an error occurs")

	_ = clusterDeployCmd.MarkFlagRequired("cluster")
}
