package cmd

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/pterm/pterm"
	"io"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var containerCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a container",
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

		container, err := getContainerContextResource(client, containerName, envId)

		if err != nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
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

		if targetContainerName == "" {
			// use same container name as the source container
			targetContainerName = container.Name
		}

		req := qovery.CloneContainerRequest{
			Name:          targetContainerName,
			EnvironmentId: targetEnvironmentId,
		}

		clonedService, res, err := client.ContainersAPI.CloneContainer(context.Background(), container.Id).CloneContainerRequest(req).Execute()

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

		utils.Println(fmt.Sprintf("Container %s cloned!", pterm.FgBlue.Sprintf(name)))
	},
}

func init() {
	containerCmd.AddCommand(containerCloneCmd)
	containerCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerCloneCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerCloneCmd.Flags().StringVarP(&targetProjectName, "target-project", "", "", "Target Project Name")
	containerCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	containerCloneCmd.Flags().StringVarP(&targetContainerName, "target-container-name", "", "", "Target Container Name")

	_ = containerCloneCmd.MarkFlagRequired("container")
}
