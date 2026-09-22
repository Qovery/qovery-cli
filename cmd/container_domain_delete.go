package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var containerDomainDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete container custom domain",
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

		container := utils.FindByContainerName(containers.GetResults(), containerName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
			os.Exit(1)
		}

		customDomains, _, err := client.ContainerCustomDomainAPI.ListContainerCustomDomain(context.Background(), container.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		customDomain := utils.FindByCustomDomainName(customDomains.GetResults(), containerCustomDomain)
		if customDomain == nil {
			utils.PrintlnError(fmt.Errorf("custom domain %s does not exist", containerCustomDomain))
			os.Exit(1)
		}

		_, err = client.ContainerCustomDomainAPI.DeleteContainerCustomDomain(context.Background(), container.Id, customDomain.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been deleted", pterm.FgBlue.Sprintf("%s", containerCustomDomain)))
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
