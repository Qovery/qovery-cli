package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var webhookListEventCmd = &cobra.Command{
	Use:   "list-event",
	Short: "List webhook events",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
		checkError(err)

		events, _, err := client.OrganizationWebhookAPI.ListWebhookEvent(context.Background(), organizationId, webhookId).Execute()
		checkError(err)

		if jsonFlag {
			utils.Println(getWebhookEventJsonOutput(events))
			return
		}

		var data [][]string
		for _, event := range events.GetResults() {
			data = append(data, []string{
				event.Id,
				event.CreatedAt.String(),
				string(event.MatchedEvent),
				string(event.Kind),
				event.TargetUrlUsed,
				fmt.Sprintf("%d", event.TargetResponseStatusCode),
				*event.TargetResponseBody.Get(),
			})
		}

		err = utils.PrintTable([]string{"ID", "Created At", "Event", "Kind", "Target URL", "Response Status Code", "Response Body"}, data)
		checkError(err)
	},
}

func getWebhookEventJsonOutput(events *qovery.WebhookEventResponseList) string {
	var results []interface{}

	for _, event := range events.GetResults() {
		results = append(results, map[string]interface{}{
			"id":                          event.Id,
			"matched_event":               string(event.MatchedEvent),
			"kind":                        string(event.Kind),
			"target_url_used":             event.TargetUrlUsed,
			"target_response_status_code": event.TargetResponseStatusCode,
			"target_response_body":        event.TargetResponseBody.Get(),
			"created_at":                  event.CreatedAt.String(),
			"payload":                     event.Request,
		})
	}

	j, err := json.Marshal(results)
	checkError(err)

	return string(j)
}

func init() {
	webhookCmd.AddCommand(webhookListEventCmd)
	webhookListEventCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	webhookListEventCmd.Flags().StringVarP(&webhookId, "webhook-id", "", "", "Webhook ID (UUID)")
	webhookListEventCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
	_ = webhookListEventCmd.MarkFlagRequired("webhook-id")
}
