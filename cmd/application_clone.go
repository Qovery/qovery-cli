package cmd

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"io"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		application, err := getApplicationContextResource(client, applicationName, envId)

		if err != nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		targetProjectId := projectId // use same project as the source project
		if targetProjectName != "" {

			targetProjectId, err = getProjectContextResourceId(client, targetProjectName, organizationId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		targetEnvironmentId := envId // use same env as the source env
		if targetEnvironmentName != "" {

			targetEnvironmentId, err = getEnvironmentContextResourceId(client, targetEnvironmentName, targetProjectId)

			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		if targetApplicationName == "" {
			// use same app name as the source app
			targetApplicationName = application.Name
		}

		req := qovery.CloneServiceRequest{
			Name:          targetApplicationName,
			EnvironmentId: targetEnvironmentId,
		}

		clonedService, res, err := client.ApplicationsAPI.CloneApplication(context.Background(), application.Id).CloneServiceRequest(req).Execute()

		if err != nil {
			// print http body error message
			if res.StatusCode != 200 {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		name := ""
		if clonedService != nil {
			name = clonedService.Name
		}

		utils.Println(fmt.Sprintf("Application %s cloned!", pterm.FgBlue.Sprintf("%s", name)))
	},
}

func init() {
	applicationCmd.AddCommand(applicationCloneCmd)
	applicationCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationCloneCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationCloneCmd.Flags().StringVarP(&targetProjectName, "target-project", "", "", "Target Project Name")
	applicationCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	applicationCloneCmd.Flags().StringVarP(&targetApplicationName, "target-application-name", "", "", "Target Application Name")

	_ = applicationCloneCmd.MarkFlagRequired("application")
}
