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

var applicationDomainEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit application custom domain",
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

		generateCertificate := !doNotGenerateCertificate
		req := qovery.CustomDomainRequest{
			Domain:              applicationCustomDomain,
			GenerateCertificate: generateCertificate,
			UseCdn:              &useCdn,
		}

		editedDomain, _, err := client.CustomDomainAPI.EditCustomDomain(context.Background(), application.Id, customDomain.Id).CustomDomainRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been edited (generate certificate: %s)", pterm.FgBlue.Sprintf(editedDomain.Domain), pterm.FgBlue.Sprintf(strconv.FormatBool(editedDomain.GenerateCertificate))))
	},
}

func init() {
	applicationDomainCmd.AddCommand(applicationDomainEditCmd)
	applicationDomainEditCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDomainEditCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDomainEditCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDomainEditCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationDomainEditCmd.Flags().StringVarP(&applicationCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")
	applicationDomainEditCmd.Flags().BoolVarP(&doNotGenerateCertificate, "do-not-generate-certificate", "", false, "Do Not Generate Certificate")
	applicationDomainEditCmd.Flags().BoolVarP(&useCdn, "is-behind-a-cdn", "", false, "Custom Domain is behind a CDN")

	_ = applicationDomainEditCmd.MarkFlagRequired("application")
	_ = applicationDomainEditCmd.MarkFlagRequired("domain")
}
