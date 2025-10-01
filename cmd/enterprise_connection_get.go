package cmd

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/pkg/usercontext"
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

	_ = enterpriseConnectionGetCmd.MarkFlagRequired("organization")
	_ = enterpriseConnectionGetCmd.MarkFlagRequired("connection")

	enterpriseConnectionCmd.AddCommand(enterpriseConnectionGetCmd)
}

func getEnterpriseConnection() {
	// Get access token and client
	tokenType, token, err := utils.GetAccessToken()
	checkError(err)

	client := utils.GetQoveryClient(tokenType, token)

	targetOrganizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	checkError(err)

	enterpriseConnection, _, err := client.OrganizationEnterpriseConnectionAPI.GetOrganizationEnterpriseConnection(context.Background(), targetOrganizationId, connectionName).Execute()
	checkError(err)

	// TODO: Do not need to check that it's a UUID, only the name is required and we need to check inside the Organization Available ROles thus
	availableRoles, _, err := client.OrganizationMainCallsAPI.ListOrganizationAvailableRoles(context.Background(), targetOrganizationId).Execute()
	checkError(err)

	// Create hashmap of qoveryRoles to be [uuid] = [role-name]
	roleMap := make(map[string]string)
	for _, role := range availableRoles.Results {
		roleMap[role.Id] = role.Name
	}

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
	utils.Println("=============")
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
	utils.Println("==============")
	err = utils.PrintTable([]string{"Qovery Role", "Your IDPs roles"}, data)
	if err != nil {
		utils.PrintlnError(err)
		return
	}
}
