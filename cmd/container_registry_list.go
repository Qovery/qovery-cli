package cmd

import (
	"context"
	"encoding/json"

	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var containerRegistryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List container registries",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		utils.CheckError(err)

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
		utils.CheckError(err)

		registries, _, err := client.ContainerRegistriesAPI.ListContainerRegistry(context.Background(), organizationId).Execute()
		utils.CheckError(err)

		if jsonFlag {
			utils.Println(getContainerRegistryJsonOutput(registries.GetResults()))
			return
		}

		var data [][]string
		for _, registry := range registries.GetResults() {
			url := ""
			if registry.Url != nil {
				url = *registry.Url
			}
			kind := ""
			if registry.Kind != nil {
				kind = string(*registry.Kind)
			}
			name := ""
			if registry.Name != nil {
				name = *registry.Name
			}
			data = append(data, []string{registry.Id, name, kind, url})
		}

		utils.CheckError(utils.PrintTable([]string{"Id", "Name", "Kind", "URL"}, data))
	},
}

func getContainerRegistryJsonOutput(registries []qovery.ContainerRegistryResponse) string {
	var results []interface{}
	for _, registry := range registries {
		url := ""
		if registry.Url != nil {
			url = *registry.Url
		}
		kind := ""
		if registry.Kind != nil {
			kind = string(*registry.Kind)
		}
		name := ""
		if registry.Name != nil {
			name = *registry.Name
		}
		results = append(results, map[string]interface{}{
			"id":   registry.Id,
			"name": name,
			"kind": kind,
			"url":  url,
		})
	}

	j, err := json.Marshal(results)
	utils.CheckError(err)

	return string(j)
}

func init() {
	containerRegistryCmd.AddCommand(containerRegistryListCmd)
	containerRegistryListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerRegistryListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
