package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"os"
)

var clusterLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock a cluster",
	Run: func(cmd *cobra.Command, args []string) {
		lockCluster()
	},
}

func init() {
	clusterLockCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	clusterLockCmd.Flags().StringVarP(&lockReason, "reason", "r", "", "Reason")
	clusterLockCmd.Flags().Int32VarP(&lockTtlInDays, "ttl-in-days", "d", -1, "TTL in days")

	_ = clusterLockCmd.MarkFlagRequired("cluster-id")
	_ = clusterLockCmd.MarkFlagRequired("reason")

	clusterCmd.AddCommand(clusterLockCmd)
}

func lockCluster() {
	var ttlInDays *int32 = nil
	if lockTtlInDays != -1 {
		ttlInDays = &lockTtlInDays
	}

	if utils.Validate("lock") {
		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)

		lockClusterRequest := qovery.ClusterLockRequest{
			Reason:    lockReason,
			TtlInDays: ttlInDays,
		}

		_, http, err := client.ClustersAPI.LockCluster(context.Background(), clusterId).ClusterLockRequest(lockClusterRequest).Execute()
		if err != nil {
			utils.PrintlnError(err)
			result, _ := io.ReadAll(http.Body)
			LogDetail(result)
			os.Exit(1)
		}

		fmt.Println("Cluster locked.")
	}
}

func LogDetail(result []byte) {
	var response struct {
		Detail string `json:"detail"`
	}

	if err := json.Unmarshal(result, &response); err != nil {
		log.Error("", result)
	} else {
		if response.Detail != "" {
			log.Error("Error detail: ", response.Detail)
		} else {
			log.Error("", result)
		}
	}
}
