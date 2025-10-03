package cmd

import (
	"fmt"

	"github.com/qovery/qovery-cli/pkg/enterpriseconnection"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	enterpriseConnectionGroupMappingsDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete enterprise connection group mapping",
		Run: func(cmd *cobra.Command, args []string) {
			deleteEnterpriseConnectionGroupMapping()
		},
	}
)

func init() {
	enterpriseConnectionGroupMappingsDeleteCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	enterpriseConnectionGroupMappingsDeleteCmd.Flags().StringVarP(&connectionName, "connection", "c", "", "Connection Name")
	enterpriseConnectionGroupMappingsDeleteCmd.Flags().StringVarP(&qoveryRole, "qovery-role", "q", "", "Qovery role to target")

	_ = enterpriseConnectionGroupMappingsDeleteCmd.MarkFlagRequired("connection")

	enterpriseConnectionGroupMappingsCmd.AddCommand(enterpriseConnectionGroupMappingsDeleteCmd)
}

func deleteEnterpriseConnectionGroupMapping() {
	service, err := enterpriseconnection.NewEnterpriseConnectionService(organizationName)
	checkError(err)

	// First, fetch the existing connection to get current values
	existingConnection, err := service.GetEnterpriseConnection(connectionName)
	checkError(err)

	// Resolve role name to ID
	providedRoleNameOrCustomRoleId, err := service.ResolveProvidedRoleNameOrCustomRoleId(qoveryRole)
	checkError(err)

	groupMappingsToUpdate := existingConnection.GroupMappings

	// Check if the qoveryRole exists in group mappings
	if _, exists := groupMappingsToUpdate[providedRoleNameOrCustomRoleId]; !exists {
		utils.PrintlnInfo(fmt.Sprintf("The role '%s' is not present in group mappings, skipping.", qoveryRole))
		return
	}

	// Remove the qoveryRole from group mappings
	delete(groupMappingsToUpdate, providedRoleNameOrCustomRoleId)

	dto := enterpriseconnection.CreateConnectionUpdateDto(existingConnection.DefaultRole, existingConnection.EnforceGroupSync, groupMappingsToUpdate)
	enterpriseConnection, err := service.UpdateEnterpriseConnection(connectionName, dto)
	checkError(err)

	err = service.DisplayGroupMappingsTable(enterpriseConnection.GroupMappings)
	checkError(err)
}
