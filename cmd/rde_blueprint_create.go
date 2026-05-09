package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeBlueprintCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Register a project as an RDE blueprint",
	Long: `Register an existing project as an RDE blueprint by setting the
BLUEPRINT_PROJECT_ID project-level variable and BLUEPRINT_KEY on the
first DEVELOPMENT environment.

The project must already exist and contain at least one environment.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		// Find the project by name
		project, err := rdeFindProjectByName(client, orgId, rdeBlueprintProjectName)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("project %s not found in organization", rdeBlueprintProjectName))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// Check if already registered as a blueprint
		vars, err := utils.ListProjectVariables(client, project.Id)
		if err == nil {
			existing := utils.FindEnvironmentVariableByKey(rdeBlueprintProjectIdVar, vars)
			if existing != nil {
				utils.PrintlnInfo(fmt.Sprintf("Project %s is already registered as a blueprint", rdeBlueprintProjectName))
				return
			}
		}

		// Step 1: Create project-level env var BLUEPRINT_PROJECT_ID = projectId
		utils.Println(fmt.Sprintf("Step 1/2: Setting %s on project %s...", rdeBlueprintProjectIdVar, rdeBlueprintProjectName))
		err = utils.CreateProjectVariable(client, project.Id, rdeBlueprintProjectIdVar, project.Id, false)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("failed to create project variable %s: %w", rdeBlueprintProjectIdVar, err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// Step 2: Find first environment and set BLUEPRINT_KEY = projectId
		utils.Println("Step 2/2: Setting BLUEPRINT_KEY on environment...")
		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("failed to list environments: %w", err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envResults := environments.GetResults()
		if len(envResults) == 0 {
			utils.PrintlnError(fmt.Errorf("project %s has no environments - create at least one environment first", rdeBlueprintProjectName))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// Prefer the first DEVELOPMENT environment, fallback to first env
		var targetEnv *qovery.Environment
		for _, env := range envResults {
			if env.Mode == qovery.ENVIRONMENTMODEENUM_DEVELOPMENT {
				targetEnv = &env
				break
			}
		}
		if targetEnv == nil {
			targetEnv = &envResults[0]
		}

		err = utils.CreateEnvironmentVariable(client, project.Id, targetEnv.Id, rdeBlueprintKeyVar, project.Id, false)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("failed to create environment variable %s: %w", rdeBlueprintKeyVar, err))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("")
		utils.Println(fmt.Sprintf("Blueprint registered successfully!"))
		utils.Println(fmt.Sprintf("  Project:     %s (%s)", project.Name, project.Id))
		utils.Println(fmt.Sprintf("  Environment: %s (%s)", targetEnv.Name, targetEnv.Id))
		utils.Println(fmt.Sprintf("  Console:     https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, project.Id, targetEnv.Id))
	},
}

func init() {
	rdeBlueprintCmd.AddCommand(rdeBlueprintCreateCmd)
	rdeBlueprintCreateCmd.Flags().StringVarP(&rdeBlueprintProjectName, "project", "p", "", "Project Name to register as a blueprint")
	rdeBlueprintCreateCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")

	_ = rdeBlueprintCreateCmd.MarkFlagRequired("project")
}
