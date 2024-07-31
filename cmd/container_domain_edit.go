package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"
	"strconv"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerDomainEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit container custom domain",
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

		containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

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

		customDomains, _, err := client.ContainerCustomDomainAPI.ListContainerCustomDomain(context.Background(), container.Id).Execute()

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

		generateCertificate := !doNotGenerateCertificate
		req := qovery.CustomDomainRequest{
			Domain:              containerCustomDomain,
			GenerateCertificate: generateCertificate,
			UseCdn:              &useCdn,
		}

		editedDomain, _, err := client.ContainerCustomDomainAPI.EditContainerCustomDomain(context.Background(), container.Id, customDomain.Id).CustomDomainRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been edited (generate certificate: %s)", pterm.FgBlue.Sprintf(editedDomain.Domain), pterm.FgBlue.Sprintf(strconv.FormatBool(editedDomain.GenerateCertificate))))
	},
}

func init() {
	containerDomainCmd.AddCommand(containerDomainEditCmd)
	containerDomainEditCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDomainEditCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDomainEditCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDomainEditCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDomainEditCmd.Flags().StringVarP(&containerCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")
	containerDomainEditCmd.Flags().BoolVarP(&doNotGenerateCertificate, "do-not-generate-certificate", "", false, "Do Not Generate Certificate")
	containerDomainEditCmd.Flags().BoolVarP(&useCdn, "is-behind-a-cdn", "", false, "Custom Domain is behind a CDN")

	_ = containerDomainEditCmd.MarkFlagRequired("container")
	_ = containerDomainEditCmd.MarkFlagRequired("domain")
}
