package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	organizationPlan string

	// kept in sync with the Plan enum of q-core
	adminOrganizationPlans = []string{
		"FREE",
		"PROFESSIONAL",
		"TEAM",
		"BUSINESS",
		"ENTERPRISE",
		"USER_2025",
		"TEAM_2025",
		"BUSINESS_2025",
		"ENTERPRISE_2025",
		"STARTER_2026",
		"GROWTH_2026",
		"SCALE_2026",
	}

	adminGetOrganizationPlanCmd = &cobra.Command{
		Use:   "get-organization-plan",
		Short: "Get the plan of an organization stored in the Qovery database",
		Long: `Get the plan of an organization as stored in the Qovery database, with its Chargebee subscription ID so both can be compared.

Example:
  qovery admin get-organization-plan --organization-id "xxx-xxx-xxx"
`,
		Run: func(cmd *cobra.Command, args []string) {
			getOrganizationPlan()
		},
	}

	adminUpdateOrganizationPlanCmd = &cobra.Command{
		Use:   "update-organization-plan",
		Short: "Force the plan of an organization without updating Chargebee",
		Long: fmt.Sprintf(`Force the plan of an organization in the Qovery database, without touching Chargebee.

The Chargebee subscription is left untouched: the organization keeps being invoiced on its current item price until someone updates Chargebee by hand. Use it for organizations billed out-of-band, and fix Chargebee yourself otherwise.

Accepted plans: %s

Example:
  qovery admin update-organization-plan --organization-id "xxx-xxx-xxx" --plan "ENTERPRISE_2025"
`, strings.Join(adminOrganizationPlans, ", ")),
		Run: func(cmd *cobra.Command, args []string) {
			updateOrganizationPlan()
		},
	}
)

type adminOrganizationPlanResponse struct {
	OrganizationId    string  `json:"organization_id"`
	Name              string  `json:"name"`
	Plan              string  `json:"plan"`
	BillingExternalId *string `json:"billing_external_id"`
}

type adminForceOrganizationPlanResponse struct {
	OrganizationId string `json:"organization_id"`
	Plan           string `json:"plan"`
	PreviousPlan   string `json:"previous_plan"`
	Warning        string `json:"warning"`
}

func init() {
	adminGetOrganizationPlanCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID (required)")
	_ = adminGetOrganizationPlanCmd.MarkFlagRequired("organization-id")

	adminUpdateOrganizationPlanCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID (required)")
	adminUpdateOrganizationPlanCmd.Flags().StringVarP(&organizationPlan, "plan", "p", "", "Plan to force (required)")
	_ = adminUpdateOrganizationPlanCmd.MarkFlagRequired("organization-id")
	_ = adminUpdateOrganizationPlanCmd.MarkFlagRequired("plan")

	adminCmd.AddCommand(adminGetOrganizationPlanCmd)
	adminCmd.AddCommand(adminUpdateOrganizationPlanCmd)
}

func getOrganizationPlan() {
	body := doAdminOrganizationPlanRequest(http.MethodGet, nil)

	var response adminOrganizationPlanResponse
	if err := json.Unmarshal(body, &response); err != nil {
		utils.PrintlnError(fmt.Errorf("failed to parse response: %w", err))
		os.Exit(1)
	}

	billingExternalId := "<none>"
	if response.BillingExternalId != nil && *response.BillingExternalId != "" {
		billingExternalId = *response.BillingExternalId
	}

	err := utils.PrintTable(
		[]string{"Organization ID", "Name", "Plan", "Chargebee Subscription ID"},
		[][]string{{response.OrganizationId, response.Name, response.Plan, billingExternalId}},
	)
	checkError(err)
}

func updateOrganizationPlan() {
	plan := strings.ToUpper(strings.TrimSpace(organizationPlan))
	if !slices.Contains(adminOrganizationPlans, plan) {
		utils.PrintlnError(fmt.Errorf("invalid plan %s, must be one of: %s", organizationPlan, strings.Join(adminOrganizationPlans, ", ")))
		os.Exit(1)
	}

	requestBody, err := json.Marshal(struct {
		Plan string `json:"plan"`
	}{Plan: plan})
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to marshal request body: %w", err))
		os.Exit(1)
	}

	body := doAdminOrganizationPlanRequest(http.MethodPut, requestBody)

	var response adminForceOrganizationPlanResponse
	if err := json.Unmarshal(body, &response); err != nil {
		utils.PrintlnError(fmt.Errorf("failed to parse response: %w", err))
		os.Exit(1)
	}

	err = utils.PrintTable(
		[]string{"Organization ID", "Previous Plan", "Plan"},
		[][]string{{response.OrganizationId, response.PreviousPlan, response.Plan}},
	)
	checkError(err)

	utils.PrintlnInfo(response.Warning)
}

func doAdminOrganizationPlanRequest(method string, requestBody []byte) []byte {
	if organizationId == "" {
		utils.PrintlnError(errors.New("organization ID is required"))
		os.Exit(1)
	}

	tokenType, token, err := utils.GetAccessToken(false)
	checkError(err)

	var payload io.Reader
	if requestBody != nil {
		payload = bytes.NewBuffer(requestBody)
	}

	endpoint := fmt.Sprintf("%s/organization/%s/plan", utils.GetAdminUrl(), url.PathEscape(organizationId))
	req, err := http.NewRequest(method, endpoint, payload)
	checkError(err)

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	checkError(err)
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	checkError(err)

	if res.StatusCode >= 400 {
		utils.PrintlnError(fmt.Errorf("request failed (status=%d): %s", res.StatusCode, string(body)))
		os.Exit(1)
	}

	return body
}
