package cmd

import (
	"context"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var (
	adminDeleteClusterCmd = &cobra.Command{
		Use:   "force-delete-cluster",
		Short: "Force delete cluster by id (only Qovery DB side, without calling the engine)",
		Run: func(cmd *cobra.Command, args []string) {
			deleteClusterById(cmd)
		},
	}
)

func init() {
	adminDeleteClusterCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")
	adminDeleteClusterCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeleteClusterCmd.MarkFlagRequired("cluster")
	adminCmd.AddCommand(adminDeleteClusterCmd)
}

func deleteClusterById(cmd *cobra.Command) {
	if orgaErr != nil {
		log.Error("Invalid cluster Id")
	} else {
		utils.CheckAdminUrl()
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		orgaId, _, err := getOrganizationProjectContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_ ,err = client.ClustersAPI.DeleteCluster(context.Background(), orgaId, clusterId).DeleteMode(qovery.CLUSTERDELETEMODE_DELETE_QOVERY_CONFIG).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	}
}
