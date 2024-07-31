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

var helmDomainEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit helm custom domain",
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

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helm := utils.FindByHelmName(helms.GetResults(), helmName)

		if helm == nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		customDomains, _, err := client.HelmCustomDomainAPI.ListHelmCustomDomain(context.Background(), helm.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		customDomain := utils.FindByCustomDomainName(customDomains.GetResults(), helmCustomDomain)
		if customDomain == nil {
			utils.PrintlnError(fmt.Errorf("custom domain %s does not exist", helmCustomDomain))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		generateCertificate := !doNotGenerateCertificate
		req := qovery.CustomDomainRequest{
			Domain:              helmCustomDomain,
			GenerateCertificate: generateCertificate,
			UseCdn:              &useCdn,
		}

		editedDomain, _, err := client.HelmCustomDomainAPI.EditHelmCustomDomain(context.Background(), helm.Id, customDomain.Id).CustomDomainRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been edited (generate certificate: %s)", pterm.FgBlue.Sprintf(editedDomain.Domain), pterm.FgBlue.Sprintf(strconv.FormatBool(editedDomain.GenerateCertificate))))
	},
}

func init() {
	helmDomainCmd.AddCommand(helmDomainEditCmd)
	helmDomainEditCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDomainEditCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDomainEditCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDomainEditCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmDomainEditCmd.Flags().StringVarP(&helmCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")
	helmDomainEditCmd.Flags().BoolVarP(&doNotGenerateCertificate, "do-not-generate-certificate", "", false, "Do Not Generate Certificate")
	helmDomainEditCmd.Flags().BoolVarP(&useCdn, "is-behind-a-cdn", "", false, "Custom Domain is behind a CDN")

	_ = helmDomainEditCmd.MarkFlagRequired("helm")
	_ = helmDomainEditCmd.MarkFlagRequired("domain")
}
