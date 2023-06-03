package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerDomainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List container domains",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		containers, _, err := client.ContainersApi.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		container := utils.FindByContainerName(containers.GetResults(), containerName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		customDomains, _, err := client.ContainerCustomDomainApi.ListContainerCustomDomain(context.Background(), container.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		customDomainsSet := make(map[string]bool)
		var data [][]string

		for _, customDomain := range customDomains.GetResults() {
			customDomainsSet[customDomain.Domain] = true

			data = append(data, []string{
				customDomain.Id,
				"CUSTOM_DOMAIN",
				customDomain.Domain,
				*customDomain.ValidationDomain,
			})
		}

		links, _, err := client.ContainerMainCallsApi.ListContainerLinks(context.Background(), container.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		for _, link := range links.GetResults() {
			if link.Url != nil {
				domain := strings.ReplaceAll(*link.Url, "https://", "")
				if !customDomainsSet[domain] {
					data = append(data, []string{
						"N/A",
						"BUILT_IN_DOMAIN",
						domain,
						"N/A",
					})
				}
			}
		}

		err = utils.PrintTable([]string{"Id", "Type", "Domain", "Validation Domain"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	containerDomainCmd.AddCommand(containerDomainListCmd)
	containerDomainListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDomainListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDomainListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDomainListCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")

	_ = containerDomainListCmd.MarkFlagRequired("container")
}
