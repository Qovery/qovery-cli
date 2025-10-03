package cmd

import (
	"fmt"

	"github.com/qovery/qovery-cli/pkg/enterpriseconnection"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	enterpriseConnectionGroupMappingsAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add or modify an enterprise connection group mapping",
		Run: func(cmd *cobra.Command, args []string) {
			addEnterpriseConnectionGroupMapping()
		},
	}
)

func init() {
	enterpriseConnectionGroupMappingsAddCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	enterpriseConnectionGroupMappingsAddCmd.Flags().StringVarP(&connectionName, "connection", "c", "", "Connection Name")
	enterpriseConnectionGroupMappingsAddCmd.Flags().StringVarP(&qoveryRole, "qovery-role", "q", "", "Qovery role name to target")
	enterpriseConnectionGroupMappingsAddCmd.Flags().StringVarP(&idpGroupNames, "idp-group-names", "i", "", "Your IDP group names (comma separated)")

	_ = enterpriseConnectionGroupMappingsAddCmd.MarkFlagRequired("connection")

	enterpriseConnectionGroupMappingsCmd.AddCommand(enterpriseConnectionGroupMappingsAddCmd)
}

func addEnterpriseConnectionGroupMapping() {
	service, err := enterpriseconnection.NewEnterpriseConnectionService(organizationName)
	checkError(err)

	// First, fetch the existing connection to get current values
	existingConnection, err := service.GetEnterpriseConnection(connectionName)
	checkError(err)

	// Validate the provided role
	if err := service.ValidateRole(qoveryRole); err != nil {
		utils.PrintlnError(fmt.Errorf("this role doesn't exist in your organization: %s - %v", qoveryRole, err))
		return
	}

	// Resolve role name to ID
	providedRoleNameOrCustomRoleId, err := service.ResolveProvidedRoleNameOrCustomRoleId(qoveryRole)
	checkError(err)

	// Parse IDP group names
	idpGroupNamesArray := enterpriseconnection.ParseIdpGroupNames(idpGroupNames)

	// Update group mappings
	groupMappingsToUpdate := existingConnection.GroupMappings
	groupMappingsToUpdate[providedRoleNameOrCustomRoleId] = idpGroupNamesArray

	dto := enterpriseconnection.CreateConnectionUpdateDto(existingConnection.DefaultRole, existingConnection.EnforceGroupSync, groupMappingsToUpdate)
	enterpriseConnection, err := service.UpdateEnterpriseConnection(connectionName, dto)
	checkError(err)

	err = service.DisplayGroupMappingsTable(enterpriseConnection.GroupMappings)
	checkError(err)
}
