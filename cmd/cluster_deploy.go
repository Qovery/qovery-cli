package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"os"
	"time"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
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
		orgId, _, _, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		clusters, _, err := client.ClustersAPI.ListOrganizationCluster(context.Background(), orgId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cluster := utils.FindByClusterName(clusters.GetResults(), clusterName)

		if cluster == nil {
			utils.PrintlnError(fmt.Errorf("cluster %s not found", clusterName))
			utils.PrintlnInfo("You can list all clusters with: qovery cluster list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, res, err := client.ClustersAPI.DeployCluster(context.Background(), orgId, cluster.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)

			// print http body error message
			if res.StatusCode != 200 {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if watchFlag {
			for {
				status, _, err := client.ClustersAPI.GetClusterStatus(context.Background(), orgId, cluster.Id).Execute()
				if err != nil {
					utils.PrintlnError(err)
				}

				if utils.IsTerminalClusterState(*status.Status) {
					break
				}

				utils.Println(fmt.Sprintf("Cluster status: %s", utils.GetClusterStatusTextWithColor(status.GetStatus())))

				// sleep here to avoid too many requests
				time.Sleep(5 * time.Second)
			}

			utils.Println(fmt.Sprintf("Cluster %s deployed!", pterm.FgBlue.Sprintf(clusterName)))
		} else {
			utils.Println(fmt.Sprintf("Deploying cluster %s in progress..", pterm.FgBlue.Sprintf(clusterName)))
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
