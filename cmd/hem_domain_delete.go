package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var helmDomainDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete helm custom domain",
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

		customDomains, _, err := client.CustomDomainAPI.ListHelmCustomDomain(context.Background(), helm.Id).Execute()

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

		_, err = client.HelmCustomDomainAPI.DeleteHelmCustomDomain(context.Background(), helm.Id, customDomain.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Custom domain %s has been deleted", pterm.FgBlue.Sprintf(helmCustomDomain)))
	},
}

func init() {
	helmDomainCmd.AddCommand(helmDomainDeleteCmd)
	helmDomainDeleteCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDomainDeleteCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDomainDeleteCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDomainDeleteCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmDomainDeleteCmd.Flags().StringVarP(&helmCustomDomain, "domain", "", "", "Custom Domain <subdomain.domain.tld>")

	_ = helmDomainDeleteCmd.MarkFlagRequired("helm")
	_ = helmDomainDeleteCmd.MarkFlagRequired("domain")
}
