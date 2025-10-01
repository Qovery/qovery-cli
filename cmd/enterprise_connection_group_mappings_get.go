package cmd

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
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
	// Get access token and client
	tokenType, token, err := utils.GetAccessToken()
	checkError(err)

	client := utils.GetQoveryClient(tokenType, token)

	targetOrganizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	checkError(err)

	enterpriseConnection, _, err := client.OrganizationEnterpriseConnectionAPI.GetOrganizationEnterpriseConnection(context.Background(), targetOrganizationId, connectionName).Execute()
	checkError(err)

	// Fetch custom roles and create role mapping
	qoveryRoles, _, err := client.OrganizationCustomRoleAPI.ListOrganizationCustomRoles(context.Background(), targetOrganizationId).Execute()
	checkError(err)

	// Create hashmap of qoveryRoles to be [uuid] = [role-name]
	roleMap := make(map[string]string)
	for _, role := range qoveryRoles.Results {
		roleMap[*role.Id] = *role.Name
	}

	// Display default role
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

	err = utils.PrintTable([]string{"Qovery Role", "Your IDPs roles"}, data)
	if err != nil {
		utils.PrintlnError(err)
		return
	}
}
