package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	clusterDomain string
)

var adminClusterUpdateDomainCmd = &cobra.Command{
	Use:   "update-domain",
	Short: "Update cluster domain/managed dns for a new one. Cluster and all apps need to be re-deployed after",
	Run: func(cmd *cobra.Command, args []string) {
		updateClusterDomain()
	},
}

func init() {
	adminClusterUpdateDomainCmd.Flags().StringVar(&clusterId, "cluster-id", "", "The cluster id to target")
	adminClusterUpdateDomainCmd.Flags().StringVar(&clusterDomain, "domain", "", "The new domain for the cluster")
	adminClusterCmd.AddCommand(adminClusterUpdateDomainCmd)
}

func updateClusterDomain() {
	var err error
	if clusterId == "" {
		utils.PrintlnError(err)
		utils.PrintlnInfo("cluster-id is required")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if clusterDomain == "" {
		utils.PrintlnError(err)
		utils.PrintlnInfo("domain is required")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	err = pkg.UpdateClusterDomainName(clusterId, clusterDomain)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	utils.PrintlnInfo("domain updated successfully")
}
