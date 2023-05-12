package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerDomainCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create container custom domain",
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

		container := utils.FindByContainerName(containers.GetResults(), applicationName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", applicationName))
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

		customDomain := utils.FindByCustomDomainName(customDomains.GetResults(), applicationCustomDomain)
		if customDomain != nil {
			utils.PrintlnError(fmt.Errorf("custom domain %s already exists", applicationCustomDomain))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		req := qovery.CustomDomainRequest{
			Domain: applicationCustomDomain,
		}

		_, _, err = client.ContainerCustomDomainApi.CreateContainerCustomDomain(context.Background(), container.Id).CustomDomainRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been created", pterm.FgBlue.Sprintf(applicationCustomDomain)))
	},
}

func init() {
	containerDomainCmd.AddCommand(containerDomainCreateCmd)
	containerDomainCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDomainCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDomainCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDomainCreateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDomainCreateCmd.Flags().StringVarP(&containerCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")

	_ = containerDomainCreateCmd.MarkFlagRequired("container")
	_ = containerDomainCreateCmd.MarkFlagRequired("domain")
}
