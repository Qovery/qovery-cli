package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminOrganizationDeploymentRestrictionCmd = &cobra.Command{
		Use:   "deployment-restriction",
		Short: "Block or unblock organization deployments",
		Long: `Manage organization deployment restrictions.
This command allows you to block or unblock deployments for a specific organization.

Examples:
  qovery admin deployment-restriction --organization-id 12345678-1234-1234-1234-123456789abc --action block --message "Payment overdue"
  qovery admin deployment-restriction --organization-id 12345678-1234-1234-1234-123456789abc --action unblock`,
		Run: func(cmd *cobra.Command, args []string) {
			manageOrganizationDeploymentRestriction()
		},
	}
)

func init() {
	adminOrganizationDeploymentRestrictionCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID")
	adminOrganizationDeploymentRestrictionCmd.Flags().StringVarP(&deploymentAction, "action", "a", "", "Action to perform: block or unblock")
	adminOrganizationDeploymentRestrictionCmd.Flags().StringVarP(&restrictionMessage, "message", "m", "", "Message explaining the reason for blocking (required when action is 'block')")

	_ = adminOrganizationDeploymentRestrictionCmd.MarkFlagRequired("organization-id")
	_ = adminOrganizationDeploymentRestrictionCmd.MarkFlagRequired("action")

	adminCmd.AddCommand(adminOrganizationDeploymentRestrictionCmd)
}

var deploymentAction string
var restrictionMessage string

func manageOrganizationDeploymentRestriction() {
	// Validate action
	if deploymentAction != "block" && deploymentAction != "unblock" {
		utils.PrintlnError(fmt.Errorf("action must be either 'block' or 'unblock', got: %s", deploymentAction))
		os.Exit(1)
	}

	// Validate organization ID format (basic UUID check)
	if organizationId == "" {
		utils.PrintlnError(fmt.Errorf("organization ID is required"))
		os.Exit(1)
	}

	// Basic UUID format validation
	if len(organizationId) != 36 ||
		!strings.Contains(organizationId, "-") ||
		strings.Count(organizationId, "-") != 4 {
		utils.PrintlnError(fmt.Errorf("organization ID must be a valid UUID format (e.g., 12345678-1234-1234-1234-123456789abc)"))
		os.Exit(1)
	}

	// Validate message is provided when blocking
	if deploymentAction == "block" {
		if restrictionMessage == "" {
			utils.PrintlnError(fmt.Errorf("message is required when action is 'block'. Use --message flag to provide a reason"))
			os.Exit(1)
		}

		// Validate message length and content
		if len(strings.TrimSpace(restrictionMessage)) < 3 {
			utils.PrintlnError(fmt.Errorf("message must be at least 3 characters long"))
			os.Exit(1)
		}

		if len(restrictionMessage) > 500 {
			utils.PrintlnError(fmt.Errorf("message must be less than 500 characters"))
			os.Exit(1)
		}
	}

	// Validate that message is not provided for unblock action
	if deploymentAction == "unblock" && restrictionMessage != "" {
		utils.PrintlnError(fmt.Errorf("message should not be provided when action is 'unblock'"))
		os.Exit(1)
	}

	// Show confirmation prompt
	utils.PrintlnInfo(fmt.Sprintf("You are about to %s deployments for organization: %s", deploymentAction, organizationId))
	if deploymentAction == "block" {
		utils.PrintlnInfo(fmt.Sprintf("Reason: %s", restrictionMessage))
	}
	utils.PrintlnInfo("This action will affect ALL deployments for this organization.")

	// Ask for confirmation
	if !utils.Validate("deployment restriction") {
		utils.PrintlnInfo("Operation cancelled.")
		return
	}

	// Get access token
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	// Prepare request payload
	payload := struct {
		Action  string `json:"action"`
		Message string `json:"message,omitempty"`
	}{
		Action:  deploymentAction,
		Message: restrictionMessage,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to marshal payload: %w", err))
		os.Exit(1)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/organization/%s/deploymentRestriction", utils.GetAdminUrl(), organizationId)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to create request: %w", err))
		os.Exit(1)
	}

	// Set headers
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to execute request: %w", err))
		os.Exit(1)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			utils.PrintlnError(fmt.Errorf("failed to close response body: %w", err))
		}
	}()

	// Read response body
	body, err := io.ReadAll(res.Body)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to read response body: %w", err))
		os.Exit(1)
	}

	// Handle response based on status code
	switch res.StatusCode {
	case http.StatusOK:
		// Try to parse response for more details
		var response struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		}

		if err := json.Unmarshal(body, &response); err == nil && response.Message != "" {
			utils.PrintlnInfo(fmt.Sprintf("✅ %s", response.Message))
		} else {
			// Fallback to generic success message
			actionText := "blocked"
			if deploymentAction == "unblock" {
				actionText = "unblocked"
			}
			utils.PrintlnInfo(fmt.Sprintf("✅ Organization %s has been %s successfully", organizationId, actionText))
		}

	case http.StatusNotFound:
		utils.PrintlnError(fmt.Errorf("❌ Organization not found: %s", organizationId))
		os.Exit(1)

	case http.StatusUnauthorized:
		utils.PrintlnError(fmt.Errorf("❌ Unauthorized: You don't have permission to perform this action"))
		os.Exit(1)

	case http.StatusForbidden:
		utils.PrintlnError(fmt.Errorf("❌ Forbidden: You don't have permission to perform this action"))
		os.Exit(1)

	case http.StatusBadRequest:
		utils.PrintlnError(fmt.Errorf("❌ Bad request: %s", string(body)))
		os.Exit(1)

	default:
		utils.PrintlnError(fmt.Errorf("❌ Request failed with status %s: %s", res.Status, string(body)))
		os.Exit(1)
	}
}
