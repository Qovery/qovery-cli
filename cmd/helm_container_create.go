package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/qovery/qovery-client-go"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var helmDomainCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create helm custom domain",
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
		if customDomain != nil {
			utils.PrintlnError(fmt.Errorf("custom domain %s already exists", helmCustomDomain))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		generateCertificate := !doNotGenerateCertificate
		req := qovery.CustomDomainRequest{
			Domain:              helmCustomDomain,
			GenerateCertificate: generateCertificate,
			UseCdn:              &useCdn,
		}

		createdDomain, _, err := client.HelmCustomDomainAPI.CreateHelmCustomDomain(context.Background(), helm.Id).CustomDomainRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been created (generate certificate: %s)", pterm.FgBlue.Sprintf(createdDomain.Domain), pterm.FgBlue.Sprintf(strconv.FormatBool(createdDomain.GenerateCertificate))))
	},
}

func init() {
	helmDomainCmd.AddCommand(helmDomainCreateCmd)
	helmDomainCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDomainCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDomainCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDomainCreateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmDomainCreateCmd.Flags().StringVarP(&helmCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")
	helmDomainCreateCmd.Flags().BoolVarP(&doNotGenerateCertificate, "do-not-generate-certificate", "", false, "Do Not Generate Certificate")
	helmDomainCreateCmd.Flags().BoolVarP(&useCdn, "is-behind-a-cdn", "", false, "Custom Domain is behind a CDN")

	_ = helmDomainCreateCmd.MarkFlagRequired("helm")
	_ = helmDomainCreateCmd.MarkFlagRequired("domain")
}
