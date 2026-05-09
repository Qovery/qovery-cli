package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an RDE (environment, project, RBAC role, and API token)",
	Long: `Fully remove an RDE by:
  1. Stopping the environment (if running)
  2. Deleting the environment
  3. Deleting the project
  4. Deleting the RBAC role RDE-<name> (if exists)
  5. Deleting the API token ttl-<name> (if exists)`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		projectName := fmt.Sprintf("rde-%s", rdeName)
		utils.Println(fmt.Sprintf("Deleting RDE: %s", rdeName))

		// Find the project
		project, err := rdeFindProjectByName(client, orgId, projectName)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("RDE %s not found (no project %s)", rdeName, projectName))
			// Still try to clean up role and token
			rdeCleanupRoleAndToken(client, orgId, rdeName)
			os.Exit(1)
			panic("unreachable")
		}

		// Find environment
		environments, _, err := client.EnvironmentsAPI.ListEnvironment(ctx(), project.Id).Execute()
		if err == nil {
			for _, env := range environments.GetResults() {
				// Stop environment
				status, _ := rdeGetEnvStatus(client, env.Id)
				if status != qovery.STATEENUM_STOPPED && status != "" {
					utils.Println(fmt.Sprintf("  Stopping environment %s...", env.Name))
					_, _, _ = client.EnvironmentActionsAPI.StopEnvironment(ctx(), env.Id).Execute()
					time.Sleep(2 * time.Second)
				}

				// Delete environment
				utils.Println(fmt.Sprintf("  Deleting environment %s...", env.Name))
				_, err = client.EnvironmentMainCallsAPI.DeleteEnvironment(ctx(), env.Id).Execute()
				if err != nil {
					utils.PrintlnInfo(fmt.Sprintf("Failed to delete environment: %v", err))
				}
			}
		}

		// Delete project
		utils.Println(fmt.Sprintf("  Deleting project %s...", projectName))
		_, err = client.ProjectMainCallsAPI.DeleteProject(ctx(), project.Id).Execute()
		if err != nil {
			utils.PrintlnInfo(fmt.Sprintf("Failed to delete project: %v", err))
		}

		// Cleanup role and token
		rdeCleanupRoleAndToken(client, orgId, rdeName)

		utils.Println("")
		utils.Println(fmt.Sprintf("RDE %s fully removed.", rdeName))
	},
}

// rdeCleanupRoleAndToken removes the RBAC role and API token associated with an RDE.
func rdeCleanupRoleAndToken(client *qovery.APIClient, orgId string, name string) {
	// Delete RBAC role
	roleName := fmt.Sprintf("RDE-%s", name)
	role, _ := rdeFindCustomRoleByName(client, orgId, roleName)
	if role != nil && role.Id != nil {
		utils.Println(fmt.Sprintf("  Deleting role %s...", roleName))
		_, _ = client.OrganizationCustomRoleAPI.DeleteOrganizationCustomRole(ctx(), orgId, *role.Id).Execute()
	}

	// Delete API token
	tokenName := fmt.Sprintf("ttl-%s", name)
	tokens, _, err := client.OrganizationApiTokenAPI.ListOrganizationApiTokens(ctx(), orgId).Execute()
	if err == nil {
		for _, token := range tokens.GetResults() {
			if token.Name != nil && *token.Name == tokenName {
				if token.Id != "" {
					utils.Println(fmt.Sprintf("  Deleting API token %s...", tokenName))
					_, _ = client.OrganizationApiTokenAPI.DeleteOrganizationApiToken(ctx(), orgId, token.Id).Execute()
				}
				break
			}
		}
	}
}

func init() {
	rdeCmd.AddCommand(rdeDeleteCmd)
	rdeDeleteCmd.Flags().StringVarP(&rdeName, "name", "n", "", "RDE Name")
	rdeDeleteCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeDeleteCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch deletion status")

	_ = rdeDeleteCmd.MarkFlagRequired("name")
}
