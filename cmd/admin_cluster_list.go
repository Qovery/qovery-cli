package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var (
	adminClusterListCmd = &cobra.Command{
		Use:   "list",
		Short: "List clusters by applying any filter",
		Long: `This command is used to list clusters information using filters.
The endpoint fetched by the CLI return all clusters except the locked ones.

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

> Examples
----------
* Display every production cluster on cloud providers AWS and GCP:
qovery admin cluster list -f IsProduction=true -f ClusterType=AWS,GCP

* Display every deployed cluster on organization "FooBar":
qovery admin cluster list -f OrganizationName=FooBar -f CurrentStatus=DEPLOYED
`,
		Run: func(cmd *cobra.Command, args []string) {
			listClusters()
		},
	}
)

func init() {
	adminClusterListCmd.Flags().StringToStringVarP(&filters, "filters", "f", make(map[string]string), "Value(s) to filter the property selected separated by comma when multiple values are defined")
	adminClusterCmd.AddCommand(adminClusterListCmd)
}

func listClusters() {
	utils.CheckAdminUrl()

	listService, err := pkg.NewAdminClusterListServiceImpl(filters)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	err = pkg.ListAllClusters(listService)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}
