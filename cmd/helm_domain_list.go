package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"
	"strconv"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var helmDomainListCmd = &cobra.Command{
	Use:   "list",
	Short: "List helm domains",
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

		customDomainsSet := make(map[string]bool)
		var data [][]string

		for _, customDomain := range customDomains.GetResults() {
			customDomainsSet[customDomain.Domain] = true

			data = append(data, []string{
				customDomain.Id,
				"CUSTOM_DOMAIN",
				customDomain.Domain,
				*customDomain.ValidationDomain,
				strconv.FormatBool(customDomain.GenerateCertificate),
			})
		}

		links, _, err := client.HelmMainCallsAPI.ListHelmLinks(context.Background(), helm.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if jsonFlag {
			utils.Println(gethelmDomainJsonOutput(links.GetResults(), customDomains.GetResults()))
			return
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
						"N/A",
					})
				}
			}
		}

		err = utils.PrintTable([]string{"Id", "Type", "Domain", "Validation Domain", "Generate Certificate"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func gethelmDomainJsonOutput(links []qovery.Link, domains []qovery.CustomDomain) string {
	var results []interface{}

	for _, link := range links {
		if link.Url != nil {
			results = append(results, map[string]interface{}{
				"id":                nil,
				"type":              "BUILT_IN_DOMAIN",
				"domain":            strings.ReplaceAll(*link.Url, "https://", ""),
				"validation_domain": nil,
			})
		}
	}

	for _, domain := range domains {
		results = append(results, map[string]interface{}{
			"id":                domain.Id,
			"type":              "CUSTOM_DOMAIN",
			"domain":            domain.Domain,
			"validation_domain": *domain.ValidationDomain,
		})
	}

	j, err := json.Marshal(results)

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(j)
}

func init() {
	helmDomainCmd.AddCommand(helmDomainListCmd)
	helmDomainListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmDomainListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmDomainListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmDomainListCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmDomainListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")

	_ = helmDomainListCmd.MarkFlagRequired("helm")
}
