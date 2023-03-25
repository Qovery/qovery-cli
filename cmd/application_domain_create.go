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

var applicationDomainCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create application custom domain",
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

		applications, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		application := utils.FindByApplicationName(applications.GetResults(), applicationName)

		if application == nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		customDomains, _, err := client.CustomDomainApi.ListApplicationCustomDomain(context.Background(), application.Id).Execute()

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

		_, _, err = client.CustomDomainApi.CreateApplicationCustomDomain(context.Background(), application.Id).CustomDomainRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been created", pterm.FgBlue.Sprintf(applicationCustomDomain)))
	},
}

func init() {
	applicationDomainCmd.AddCommand(applicationDomainCreateCmd)
	applicationDomainCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDomainCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDomainCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDomainCreateCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDomainCreateCmd.Flags().StringVarP(&applicationCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")

	_ = applicationDomainCreateCmd.MarkFlagRequired("application")
	_ = applicationDomainCreateCmd.MarkFlagRequired("domain")
}
