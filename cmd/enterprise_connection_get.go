package cmd

import (
	"strings"

	"github.com/qovery/qovery-cli/pkg/enterpriseconnection"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	enterpriseConnectionGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get enterprise connection information",
		Run: func(cmd *cobra.Command, args []string) {
			getEnterpriseConnection()
		},
	}
)

func init() {
	enterpriseConnectionGetCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	enterpriseConnectionGetCmd.Flags().StringVarP(&connectionName, "connection", "c", "", "Connection Name")

	enterpriseConnectionCmd.AddCommand(enterpriseConnectionGetCmd)
}

func getEnterpriseConnection() {
	service, err := enterpriseconnection.NewEnterpriseConnectionService(organizationName)
	checkError(err)

	enterpriseConnections, err := service.ListEnterpriseConnections(connectionName)
	checkError(err)

	for i, enterpriseConnection := range enterpriseConnections {
		if i > 0 {
			utils.Println("\n" + strings.Repeat("-", 50) + "\n")
		}
		err = service.DisplayEnterpriseConnection(&enterpriseConnection)
		checkError(err)
	}
}
