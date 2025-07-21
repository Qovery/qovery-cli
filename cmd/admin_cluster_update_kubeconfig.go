package cmd

import (
	"errors"
	"os"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/pkg/cluster"
	"github.com/qovery/qovery-cli/pkg/promptuifactory"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var (
	adminClusterUpdateKubeconfigCmd = &cobra.Command{
		Use:   "kubeconfig",
		Short: "Update cluster kubeconfig",
		Run: func(cmd *cobra.Command, args []string) {
			updateClusterKubeconfig()
		},
	}
)

func init() {
	adminClusterUpdateKubeconfigCmd.Flags().StringVar(&organizationId, "organization-id", "", "The cluster's organization ")
	adminClusterUpdateKubeconfigCmd.Flags().StringVar(&clusterId, "cluster-id", "", "The cluster id to target")
	adminClusterUpdateKubeconfigCmd.Flags().StringVar(&clusterKubeconfig, "kubeconfig", "", "The cluster kubeconfig string value")
	adminClusterCmd.AddCommand(adminClusterUpdateKubeconfigCmd)
}

func updateClusterKubeconfig() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	client := utils.GetQoveryClient(tokenType, token)

	// Allow this for self managed cluster only for the time being
	cluster, err := cluster.NewClusterService(client, &promptuifactory.PromptUiFactoryImpl{}).GetClusterByID(organizationId, clusterId)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if cluster.Kubernetes == nil || *cluster.Kubernetes != qovery.KUBERNETESENUM_SELF_MANAGED {
		utils.PrintlnError(errors.New("kubeconfig update is supported for SELF MANAGED clusters only"))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	err = pkg.UpdateClusterKubeconfig(organizationId, clusterId, clusterKubeconfig)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}
