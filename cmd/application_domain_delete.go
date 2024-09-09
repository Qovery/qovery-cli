package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var applicationDomainDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete application custom domain",
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

		applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

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

		customDomains, _, err := client.CustomDomainAPI.ListApplicationCustomDomain(context.Background(), application.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		customDomain := utils.FindByCustomDomainName(customDomains.GetResults(), applicationCustomDomain)
		if customDomain == nil {
			utils.PrintlnError(fmt.Errorf("custom domain %s does not exist", applicationCustomDomain))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, err = client.CustomDomainAPI.DeleteCustomDomain(context.Background(), application.Id, customDomain.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been deleted", pterm.FgBlue.Sprintf("%s", applicationCustomDomain)))
	},
}

func init() {
	applicationDomainCmd.AddCommand(applicationDomainDeleteCmd)
	applicationDomainDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDomainDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDomainDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDomainDeleteCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDomainDeleteCmd.Flags().StringVarP(&applicationCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")

	_ = applicationDomainDeleteCmd.MarkFlagRequired("application")
	_ = applicationDomainDeleteCmd.MarkFlagRequired("domain")
}
