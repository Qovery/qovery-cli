package cmd

import (
	"github.com/qovery/qovery-cli/pkg/enterpriseconnection"
	"github.com/spf13/cobra"
)

var (
	enterpriseConnectionUpdateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update enterprise connection information",
		Run: func(cmd *cobra.Command, args []string) {
			updateEnterpriseConnection()
		},
	}
)

func init() {
	enterpriseConnectionUpdateCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	enterpriseConnectionUpdateCmd.Flags().StringVarP(&connectionName, "connection", "c", "", "Connection Name")
	enterpriseConnectionUpdateCmd.Flags().StringVarP(&defaultRole, "default-role", "r", "", "Default Role")
	enterpriseConnectionUpdateCmd.Flags().BoolVarP(&enforceGroupSync, "enforce-group-sync", "e", false, "")

	_ = enterpriseConnectionUpdateCmd.MarkFlagRequired("connection")

	enterpriseConnectionCmd.AddCommand(enterpriseConnectionUpdateCmd)
}

func updateEnterpriseConnection() {
	service, err := enterpriseconnection.NewEnterpriseConnectionService(organizationName)
	checkError(err)

	// First, fetch the existing connection to get current values
	existingConnection, err := service.GetEnterpriseConnection(connectionName)
	checkError(err)

	// Use existing default role if not provided
	providedRoleNameOrCustomRoleId := defaultRole
	if providedRoleNameOrCustomRoleId == "" {
		providedRoleNameOrCustomRoleId = existingConnection.DefaultRole
	} else {
		// Resolve role name to ID if needed
		providedRoleNameOrCustomRoleId, err = service.ResolveProvidedRoleNameOrCustomRoleId(providedRoleNameOrCustomRoleId)
		checkError(err)
	}

	dto := enterpriseconnection.CreateConnectionUpdateDto(providedRoleNameOrCustomRoleId, enforceGroupSync, existingConnection.GroupMappings)
	enterpriseConnection, err := service.UpdateEnterpriseConnection(connectionName, dto)
	checkError(err)

	err = service.DisplayEnterpriseConnection(enterpriseConnection)
	checkError(err)
}
