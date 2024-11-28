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
		Long: `This command has 2 main purposes:
* deploy / redeploy clusters: mainly used to update Qovery components (agent / charts / etc.)
* upgrade clusters: used to upgrade to next kube version supported

> Filters
---------
Apply filters using the "--filters" option: filters can be applied to one or more values separated by comma interpreted as logical OR.
The fields usable as filters are the following ones:
* OrganizationId
* OrganizationName
* OrganizationPlan
* ClusterId
* ClusterName
* ClusterType
* ClusterK8sVersion
* Mode
* IsProduction
* CurrentStatus

Not implemented yet: filtering from last deployed date or created date

> Parallel Run number
---------------------
The option "--parallel-run" (-n) specifies the number of parallel cluster deployments to be launched (default = 5)
The deployments are launched locally on the workstation, not on a server-side thread.
* if the value is > 20 the cluster autoscaler should be updated manually (the command displays a message and requires an approval to be launched)
* the maximum value cannot exceed 100

> Execution Mode
----------------
The option "--execution-mode" specifies which mode is applied on execution:
* "--execution-mode=batch" (default): deployments are triggered sequentially by batch of N parallel-runs. The next batch of deployments will be launched only after all previous batch deployments
* "--execution-mode=on-the-fly": deployments are triggered as soon as there is a slot available in a thread pool of N parallel-runs

> New K8S Version
-----------------
The option "--new-k8s-version" specifies the next kubernetes version to be applied.
When using this option, the recommendation is to have a low parallel runs number and an execution mode on batch, to be able to monitor clusters peacefully.

> Refresh Delay
---------------
The option "--refresh-delay" specifies the amount of time to wait before fetching new cluster statuses during the deployments.

> Disable Dry Run
-----------------
This option "--disable-dry-run" is mandatory to trigger the deployments

> Examples
----------
* Redeploy only 2 clusters and ensure they are non production
qovery admin cluster deploy -f ClusterName="ClusterA,ClusterB" -f IsProduction=false

* Upgrade cluster having id "80981324-b6u7-400b-97fc-e2173d46a00e" to kube version "1.28" with refreshing statuses locally every "100" seconds
qovery admin cluster deploy -f ClusterId=80981324-b6u7-400b-97fc-e2173d46a00e --new-k8s-version=1.28 --refresh-delay=100 --disable-dry-run

* Upgrade by batch of "8" parallel runs every "1.27" Kubernetes "Production" clusters on "AWS" to kubernetes version "1.28"  with refreshing statuses locally every "100" seconds
qovery admin cluster deploy -f IsProduction=true --parallel-run=8 --refresh-delay=100 -f ClusterK8sVersion=1.27 --new-k8s-version=1.28 -f ClusterType=AWS --disable-dry-run

* Redeploy by batch of "9" parallel runs every "1.27" Kubernetes clusters on "GCP" that have the last deployment status to "DEPLOYMENT_ERROR"
qovery admin cluster deploy -f ClusterType=GCP --parallel-run=9 -f ClusterK8sVersion=1.27 -f CurrentStatus=DEPLOYMENT_ERROR --disable-dry-run
`,
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
	adminClusterDeployCmd.Flags().BoolVarP(&noConfirm, "no-confirm", "c", false, "Do not prompt for confirmation")
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
	deployService, err := pkg.NewAdminClusterBatchDeployServiceImpl(dryRun, parallelRuns, refreshDelay, executionMode, newK8sVersion, noConfirm)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	err = pkg.DeployClustersByBatch(listService, deployService, noConfirm)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}
