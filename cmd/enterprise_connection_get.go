package cmd

import (
	"github.com/qovery/qovery-cli/pkg/enterpriseconnection"
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

	_ = enterpriseConnectionGetCmd.MarkFlagRequired("connection")

	enterpriseConnectionCmd.AddCommand(enterpriseConnectionGetCmd)
}

func getEnterpriseConnection() {
	service, err := enterpriseconnection.NewEnterpriseConnectionService(organizationName)
	checkError(err)

	enterpriseConnection, err := service.GetEnterpriseConnection(connectionName)
	checkError(err)

	err = service.DisplayEnterpriseConnection(enterpriseConnection)
	checkError(err)
}
