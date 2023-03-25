package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerDomainDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete container custom domain",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getContextResourcesId(client)

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

		customDomain := utils.FindByCustomDomainName(customDomains.GetResults(), containerCustomDomain)
		if customDomain == nil {
			utils.PrintlnError(fmt.Errorf("custom domain %s does not exist", containerCustomDomain))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, err = client.ContainerCustomDomainApi.DeleteContainerCustomDomain(context.Background(), container.Id, customDomain.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been deleted", pterm.FgBlue.Sprintf(containerCustomDomain)))
	},
}

func init() {
	containerDomainCmd.AddCommand(containerDomainDeleteCmd)
	containerDomainDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDomainDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDomainDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDomainDeleteCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDomainDeleteCmd.Flags().StringVarP(&containerCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")

	_ = containerDomainDeleteCmd.MarkFlagRequired("container")
	_ = containerDomainDeleteCmd.MarkFlagRequired("domain")
}
