package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var applicationDomainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List application domains",
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

		customDomainsSet := make(map[string]bool)
		var data [][]string

		for _, customDomain := range customDomains.GetResults() {
			customDomainsSet[customDomain.Domain] = true

			data = append(data, []string{
				customDomain.Id,
				"CUSTOM_DOMAIN",
				customDomain.Domain,
				*customDomain.ValidationDomain,
			})
		}

		links, _, err := client.ApplicationMainCallsApi.ListApplicationLinks(context.Background(), application.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		for _, link := range links.GetResults() {
			if link.Url != nil {
				domain := strings.ReplaceAll(*link.Url, "https://", "")
				if !customDomainsSet[domain] {
					data = append(data, []string{
						"N/A",
						"BUILT_IN_DOMAIN",
						domain,
						"N/A",
					})
				}
			}
		}

		err = utils.PrintTable([]string{"Id", "Type", "Domain", "Validation Domain"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	applicationDomainCmd.AddCommand(applicationDomainListCmd)
	applicationDomainListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationDomainListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationDomainListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationDomainListCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")

	_ = applicationDomainListCmd.MarkFlagRequired("application")
}
