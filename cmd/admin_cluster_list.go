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
	err = pkg.ListClusters(listService)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}
