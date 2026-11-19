package cmd

import (
	"context"
	"encoding/json"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"os"
)

var containerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List containers",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		if jsonFlag {
			utils.Println(getContainerJsonOutput(containers.GetResults(), statuses))
			return
		}

		err = utils.PrintTable([]string{"Id", "Name", "Type", "Status", "Last Update", "Source", "Image"},
			getContainerTableRows(containers.GetResults(), statuses))

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func getContainerTableRows(containers []qovery.ContainerResponse, statuses *qovery.EnvironmentStatuses) [][]string {
	var data [][]string

	for _, container := range containers {
		data = append(data, []string{container.Id, container.Name, "Container",
			utils.FindStatusTextWithColor(statuses.GetContainers(), container.Id), container.UpdatedAt.String(),
			utils.ContainerSourceLabel(&container), utils.ContainerImageLabel(&container)})
	}

	return data
}

func getContainerJsonOutput(containers []qovery.ContainerResponse, statuses *qovery.EnvironmentStatuses) string {
	var results []interface{}

	for _, container := range containers {
		result := map[string]interface{}{
			"id":          container.Id,
			"name":        container.Name,
			"type":        "Container",
			"status":      utils.FindStatus(statuses.GetApplications(), container.Id),
			"last_update": container.UpdatedAt.String(),
			"source":      "image",
			"registry":    container.Registry.Name,
			"image_name":  container.ImageName,
			"tag":         container.Tag,
		}
		if repository := utils.GetContainerGitRepository(&container); repository != nil {
			result["source"] = "git_repository"
			result["git_repository"] = map[string]interface{}{
				"url":    repository.Url,
				"branch": repository.GetBranch(),
			}
		}
		results = append(results, result)
	}

	j, err := json.Marshal(results)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	return string(j)
}

func init() {
	containerCmd.AddCommand(containerListCmd)
	containerListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
