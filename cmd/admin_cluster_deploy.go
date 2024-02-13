package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var (
	adminClusterDeployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy or upgrade clusters",
		Run: func(cmd *cobra.Command, args []string) {
			deployClusters()
		},
	}
	refreshDelay  int
	filters       map[string]string
	executionMode string
	newK8sVersion string
	parallelRuns  int
)

func init() {
	adminClusterDeployCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	adminClusterDeployCmd.Flags().IntVarP(&parallelRuns, "parallel-run", "n", 5, "Number of clusters to update in parallel - must be set between 1 and 20")
	adminClusterDeployCmd.Flags().IntVarP(&refreshDelay, "refresh-delay", "r", 30, "Time in seconds to wait before checking clusters status during deployment - must be between [5-120]")
	adminClusterDeployCmd.Flags().StringToStringVarP(&filters, "filters", "f", make(map[string]string), "Value(s) to filter the property selected separated by comma when multiple values are defined")
	adminClusterDeployCmd.Flags().StringVarP(&executionMode, "execution-mode", "e", "batch", "Batch execution mode - 'batch' will wait for the N deployments to be finished and ask validation to continue - 'on-the-fly' will deploy continuously as soon as a slot is available")
	adminClusterDeployCmd.Flags().StringVarP(&newK8sVersion, "new-k8s-version", "k", "", "K8S version when upgrading clusters")
	adminClusterCmd.AddCommand(adminClusterDeployCmd)

}

func deployClusters() {
	utils.CheckAdminUrl()

	// if no filter is set, enforce to select only RUNNING clusters to avoid mistakes (e.g deploying a stopped cluster)
	_, containsKey := filters["CurrentStatus"]
	if !containsKey {
		filters["CurrentStatus"] = "DEPLOYED"
	}

	listService, err := pkg.NewAdminClusterListServiceImpl(filters)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	deployService, err := pkg.NewAdminClusterBatchDeployServiceImpl(dryRun, parallelRuns, refreshDelay, executionMode, newK8sVersion)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	err = pkg.DeployClustersByBatch(listService, deployService)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}
