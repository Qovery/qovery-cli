package cmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
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

	_ = enterpriseConnectionUpdateCmd.MarkFlagRequired("organization")
	_ = enterpriseConnectionUpdateCmd.MarkFlagRequired("connection")

	enterpriseConnectionCmd.AddCommand(enterpriseConnectionUpdateCmd)
}

func updateEnterpriseConnection() {
	// Get access token and client
	tokenType, token, err := utils.GetAccessToken()
	checkError(err)

	client := utils.GetQoveryClient(tokenType, token)

	targetOrganizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	checkError(err)

	// First, fetch the existing connection to get current values
	existingConnection, _, err := client.OrganizationEnterpriseConnectionAPI.GetOrganizationEnterpriseConnection(context.Background(), targetOrganizationId, connectionName).Execute()
	checkError(err)

	availableRoles, _, err := client.OrganizationMainCallsAPI.ListOrganizationAvailableRoles(context.Background(), targetOrganizationId).Execute()
	checkError(err)

	// Create hashmap of qoveryRoles to be [uuid] = [role-name]
	roleMap := make(map[string]string)
	for _, role := range availableRoles.Results {
		roleMap[role.Id] = role.Name
	}

	// Use existing default role if not provided
	defaultRoleToSend := defaultRole
	if defaultRoleToSend == "" {
		defaultRoleToSend = existingConnection.DefaultRole
	} else {
		// Check if defaultRoleToSend is UUID. If yes, then ensures the uuid is present in hashmap of qovery roles otherwise return error
		if err := uuid.Validate(defaultRoleToSend); err == nil {
			if _, exists := roleMap[defaultRoleToSend]; !exists {
				utils.PrintlnError(fmt.Errorf("the default role UUID '%s' is not found in organization custom roles", defaultRoleToSend))
				return
			}
		}
	}

	enterpriseConnection, _, err := client.OrganizationEnterpriseConnectionAPI.UpdateOrganizationEnterpriseConnection(context.Background(), targetOrganizationId, connectionName).
		EnterpriseConnectionDto(qovery.EnterpriseConnectionDto{
			DefaultRole:      defaultRoleToSend,
			EnforceGroupSync: enforceGroupSync,
			GroupMappings:    existingConnection.GroupMappings,
		}).
		Execute()
	checkError(err)

	// Display connection settings in table format
	defaultRoleDisplay := enterpriseConnection.DefaultRole
	if err := uuid.Validate(enterpriseConnection.DefaultRole); err == nil {
		if roleName, exists := roleMap[enterpriseConnection.DefaultRole]; exists {
			defaultRoleDisplay = roleName
		}
	}

	settingsData := [][]string{
		{defaultRoleDisplay, strconv.FormatBool(enterpriseConnection.EnforceGroupSync)},
	}

	utils.Println("Configuration:")
	err = utils.PrintTable([]string{"Default Role", "Enforce Sync Group"}, settingsData)
	if err != nil {
		utils.PrintlnError(err)
		return
	}

	var data [][]string
	for qoveryGroup, idpGroups := range enterpriseConnection.GroupMappings {
		idpGroupsStr := strings.Join(idpGroups, ", ")

		// Check if qoveryGroup is UUID. If yes, then replace with role name
		displayName := qoveryGroup
		if err := uuid.Validate(qoveryGroup); err == nil {
			if roleName, exists := roleMap[qoveryGroup]; exists {
				displayName = roleName
			}
		}

		data = append(data, []string{displayName, idpGroupsStr})
	}

	// Sort data by qoveryGroup (first column)
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	utils.Println("Group Mappings:")
	err = utils.PrintTable([]string{"Qovery Role", "Your IDPs roles"}, data)
	if err != nil {
		utils.PrintlnError(err)
		return
	}
}
