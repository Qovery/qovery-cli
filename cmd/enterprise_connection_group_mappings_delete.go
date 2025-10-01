package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
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

	_ = enterpriseConnectionGroupMappingsDeleteCmd.MarkFlagRequired("organization")
	_ = enterpriseConnectionGroupMappingsDeleteCmd.MarkFlagRequired("connection")

	enterpriseConnectionGroupMappingsCmd.AddCommand(enterpriseConnectionGroupMappingsDeleteCmd)
}

func deleteEnterpriseConnectionGroupMapping() {
	// Get access token and client
	tokenType, token, err := utils.GetAccessToken()
	checkError(err)

	client := utils.GetQoveryClient(tokenType, token)

	targetOrganizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	checkError(err)

	// First, fetch the existing connection to get current values
	existingConnection, _, err := client.OrganizationEnterpriseConnectionAPI.GetOrganizationEnterpriseConnection(context.Background(), targetOrganizationId, connectionName).Execute()
	checkError(err)

	allRoles, _, err := client.OrganizationMainCallsAPI.ListOrganizationAvailableRoles(context.Background(), targetOrganizationId).Execute()
	checkError(err)

	// Create hashmap of all roles to be [role-name] = [uuid]
	allRolesIdsByNames := make(map[string]string)
	for _, role := range allRoles.Results {
		allRolesIdsByNames[strings.ToLower(role.Name)] = role.Id
	}

	customRoles, _, err := client.OrganizationCustomRoleAPI.ListOrganizationCustomRoles(context.Background(), targetOrganizationId).Execute()
	checkError(err)

	// Create hashmap of qoveryRoles to be [role-name] = [uuid]
	customRoleIdsByName := make(map[string]string)
	for _, role := range customRoles.Results {
		customRoleIdsByName[strings.ToLower(*role.Name)] = *role.Id
	}

	var providedRoleNameOrCustomRoleIdToAdd string
	if _, exists := customRoleIdsByName[strings.ToLower(qoveryRole)]; exists {
		providedRoleNameOrCustomRoleIdToAdd = customRoleIdsByName[strings.ToLower(qoveryRole)]
	} else {
		providedRoleNameOrCustomRoleIdToAdd = strings.ToLower(qoveryRole)
	}

	groupMappingsToUpdate := existingConnection.GroupMappings

	// Check if the qoveryRole exists in group mappings
	if _, exists := groupMappingsToUpdate[providedRoleNameOrCustomRoleIdToAdd]; !exists {
		utils.PrintlnInfo(fmt.Sprintf("The role '%s' is not present in group mappings, skipping.", qoveryRole))
		return
	}

	// Remove the qoveryRole from group mappings
	delete(groupMappingsToUpdate, providedRoleNameOrCustomRoleIdToAdd)

	enterpriseConnection, _, err := client.OrganizationEnterpriseConnectionAPI.UpdateOrganizationEnterpriseConnection(context.Background(), targetOrganizationId, connectionName).
		EnterpriseConnectionDto(qovery.EnterpriseConnectionDto{
			DefaultRole:      existingConnection.DefaultRole,
			EnforceGroupSync: existingConnection.EnforceGroupSync,
			GroupMappings:    groupMappingsToUpdate,
		}).
		Execute()
	checkError(err)

	// Create hashmap of all roles to be [role-name] = [uuid]
	customRoleNamesById := make(map[string]string)
	for _, role := range customRoles.Results {
		customRoleNamesById[*role.Id] = *role.Name
	}

	// Display default role
	var data [][]string
	for qoveryTargetRole, idpGroups := range enterpriseConnection.GroupMappings {
		idpGroupsStr := strings.Join(idpGroups, ", ")

		// Check if qoveryTargetRole is UUID. If yes, then replace with custom role name
		displayName := qoveryTargetRole
		if err := uuid.Validate(qoveryTargetRole); err == nil {
			if roleName, exists := customRoleNamesById[qoveryTargetRole]; exists {
				displayName = roleName
			}
		}

		data = append(data, []string{displayName, idpGroupsStr})
	}

	// Sort data by qoveryGroup (first column)
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	err = utils.PrintTable([]string{"Qovery Role", "Your IDPs roles"}, data)
	if err != nil {
		utils.PrintlnError(err)
		return
	}
}
