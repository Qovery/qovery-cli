package cmd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhooks",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
		checkError(err)

		webhooks, _, err := client.OrganizationWebhookAPI.ListOrganizationWebHooks(context.Background(), organizationId).Execute()
		checkError(err)

		if jsonFlag {
			utils.Println(getWebhookListJsonOutput(webhooks))
			return
		}

		var data [][]string
		for _, webhook := range webhooks.GetResults() {
			kind := ""
			if webhook.Kind != nil {
				kind = string(*webhook.Kind)
			}

			targetUrl := ""
			if webhook.TargetUrl != nil {
				targetUrl = *webhook.TargetUrl
			}

			description := ""
			if webhook.Description != nil {
				description = *webhook.Description
			}

			enabled := "false"
			if webhook.Enabled != nil && *webhook.Enabled {
				enabled = "true"
			}

			events := ""
			if len(webhook.Events) > 0 {
				eventStrs := make([]string, len(webhook.Events))
				for i, event := range webhook.Events {
					eventStrs[i] = string(event)
				}
				events = strings.Join(eventStrs, ", ")
			}

			data = append(data, []string{
				webhook.Id,
				description,
				kind,
				targetUrl,
				enabled,
				events,
			})
		}

		err = utils.PrintTable([]string{"ID", "Description", "Kind", "Target URL", "Enabled", "Events"}, data)
		checkError(err)
	},
}

func getWebhookListJsonOutput(webhooks *qovery.OrganizationWebhookResponseList) string {
	var results []interface{}

	for _, webhook := range webhooks.GetResults() {
		webhookMap := map[string]interface{}{
			"id":         webhook.Id,
			"created_at": webhook.CreatedAt.String(),
		}

		if webhook.UpdatedAt != nil {
			webhookMap["updated_at"] = webhook.UpdatedAt.String()
		}

		if webhook.Description != nil {
			webhookMap["description"] = *webhook.Description
		}

		if webhook.Kind != nil {
			webhookMap["kind"] = string(*webhook.Kind)
		}

		if webhook.TargetUrl != nil {
			webhookMap["target_url"] = *webhook.TargetUrl
		}

		if webhook.Enabled != nil {
			webhookMap["enabled"] = *webhook.Enabled
		}

		if len(webhook.Events) > 0 {
			events := make([]string, len(webhook.Events))
			for i, event := range webhook.Events {
				events[i] = string(event)
			}
			webhookMap["events"] = events
		}

		if len(webhook.ProjectNamesFilter) > 0 {
			webhookMap["project_names_filter"] = webhook.ProjectNamesFilter
		}

		if len(webhook.EnvironmentTypesFilter) > 0 {
			envTypes := make([]string, len(webhook.EnvironmentTypesFilter))
			for i, envType := range webhook.EnvironmentTypesFilter {
				envTypes[i] = string(envType)
			}
			webhookMap["environment_types_filter"] = envTypes
		}

		results = append(results, webhookMap)
	}

	j, err := json.Marshal(results)
	checkError(err)

	return string(j)
}

func init() {
	webhookCmd.AddCommand(webhookListCmd)
	webhookListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	webhookListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
