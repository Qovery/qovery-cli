package cmd

import (
	"github.com/qovery/qovery-cli/pkg/enterpriseconnection"
	"github.com/spf13/cobra"
)

var (
	enterpriseConnectionGroupMappingsGetCmd = &cobra.Command{
		Use:   "get",
		Short: "Get enterprise connection group mappings",
		Run: func(cmd *cobra.Command, args []string) {
			getEnterpriseConnectionGroupMappings()
		},
	}
)

func init() {
	enterpriseConnectionGroupMappingsGetCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	enterpriseConnectionGroupMappingsGetCmd.Flags().StringVarP(&connectionName, "connection", "c", "", "Connection Name")

	_ = enterpriseConnectionGroupMappingsGetCmd.MarkFlagRequired("organization")
	_ = enterpriseConnectionGroupMappingsGetCmd.MarkFlagRequired("connection")

	enterpriseConnectionGroupMappingsCmd.AddCommand(enterpriseConnectionGroupMappingsGetCmd)
}

func getEnterpriseConnectionGroupMappings() {
	service, err := enterpriseconnection.NewEnterpriseConnectionService(organizationName)
	checkError(err)

	enterpriseConnection, err := service.GetEnterpriseConnection(connectionName)
	checkError(err)

	err = service.DisplayGroupMappingsTable(enterpriseConnection.GroupMappings)
	checkError(err)
}
