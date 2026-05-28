package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	billingExternalId                         string
	adminOrganizationUpdateBillingExternalId = &cobra.Command{
		Use:   "update-billing-external-id",
		Short: "Update the billing external ID (Chargebee subscription ID) of an organization",
		Long: `Update the billing external ID of an organization. The billing external ID is the Chargebee subscription ID.

Example:
  qovery admin update-billing-external-id --organization-id "xxx-xxx-xxx" --billing-external-id "AzyXZ8T0EI4jB4AZf"
`,
		Run: func(cmd *cobra.Command, args []string) {
			updateOrganizationBillingExternalId()
		},
	}
)

func init() {
	adminOrganizationUpdateBillingExternalId.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID (required)")
	adminOrganizationUpdateBillingExternalId.Flags().StringVarP(&billingExternalId, "billing-external-id", "b", "", "Chargebee subscription ID (required)")

	_ = adminOrganizationUpdateBillingExternalId.MarkFlagRequired("organization-id")
	_ = adminOrganizationUpdateBillingExternalId.MarkFlagRequired("billing-external-id")

	adminCmd.AddCommand(adminOrganizationUpdateBillingExternalId)
}

func updateOrganizationBillingExternalId() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	type requestBody struct {
		BillingExternalId string `json:"billing_external_id"`
	}

	bodyBytes, err := json.Marshal(requestBody{BillingExternalId: billingExternalId})
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to marshal request body: %w", err))
		os.Exit(1)
	}

	url := utils.GetAdminUrl() + "/organization/" + organizationId + "/billingExternalId"
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to create request: %w", err))
		os.Exit(1)
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to execute request: %w", err))
		os.Exit(1)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		utils.PrintlnError(fmt.Errorf("request failed (status=%d): %s", res.StatusCode, string(body)))
		os.Exit(1)
	}

	utils.Println(fmt.Sprintf("✅ Successfully updated billing external ID for organization %s", organizationId))
}
