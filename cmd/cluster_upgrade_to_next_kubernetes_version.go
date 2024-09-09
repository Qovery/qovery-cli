package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
)

var clusterUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade a cluster to next kubernetes version available for the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		orgId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
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

		status, _, err := client.ClustersAPI.GetClusterStatus(context.Background(), orgId, cluster.Id).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
		if status.NextK8sAvailableVersion.Get() == nil {
			utils.PrintlnError(fmt.Errorf("no available kubernetes version to upgrade to for this cluster"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("A new kubernetes version `%s` is available for your cluster %s." /**status.NextK8sAvailableVersion.Get()*/, "", clusterName))
		if !proceedWithoutConfirmation {
			prompt := promptui.Select{
				Label: "Do you want to proceed with cluster upgrade? [Yes/No]",
				Items: []string{"Yes", "No"},
			}
			_, upgradePromptResult, err := prompt.Run()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}

			if strings.ToLower(strings.Trim(upgradePromptResult, " ")) != "yes" {
				utils.Println("Cluster upgrade aborted")
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		} else {
			utils.Println("Skipping confirmation, proceeding with cluster upgrade..")
		}

		_, res, err := client.ClustersAPI.UpgradeCluster(context.Background(), cluster.Id).Execute()
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

			utils.Println(fmt.Sprintf("Cluster %s upgraded!", pterm.FgBlue.Sprintf("%s", clusterName)))
		} else {
			utils.Println(fmt.Sprintf("Upgrading cluster %s in progress..", pterm.FgBlue.Sprintf("%s", clusterName)))
		}
	},
}

var proceedWithoutConfirmation bool = false

func init() {
	clusterCmd.AddCommand(clusterUpgradeCmd)
	clusterUpgradeCmd.Flags().BoolVarP(&proceedWithoutConfirmation, "skip-confirmation", "y", false, "Skip prompt confirmation if passed")
	clusterUpgradeCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	clusterUpgradeCmd.Flags().StringVarP(&clusterName, "cluster", "n", "", "Cluster Name")
	clusterUpgradeCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch cluster status until it's ready or an error occurs")

	_ = clusterUpgradeCmd.MarkFlagRequired("cluster")
}
