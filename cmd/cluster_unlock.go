package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"io"
	"os"
)

var clusterUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock a cluster",
	Run: func(cmd *cobra.Command, args []string) {
		unlockCluster()
	},
}

func init() {
	clusterUnlockCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	_ = clusterLockCmd.MarkFlagRequired("cluster-id")

	clusterCmd.AddCommand(clusterUnlockCmd)
}

func unlockCluster() {
	if utils.Validate("unlock") {
		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)

		http, err := client.ClustersAPI.UnlockCluster(context.Background(), clusterId).Execute()
		if err != nil {
			utils.PrintlnError(err)
			result, _ := io.ReadAll(http.Body)
			LogDetail(result)
			os.Exit(1)
		}
		fmt.Println("Cluster unlocked.")
	}
}
