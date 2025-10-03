package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	newOwnerUserId                      string
	newOwnerEmail                       string
	authProvider                        string
	adminTransferOrganizationOwnership = &cobra.Command{
		Use:   "transfer-ownership",
		Short: "Transfer organization ownership to another user",
		Long: `Transfer organization ownership to another user by providing the organization ID and either the new owner's user ID or email.

Example:
  qovery admin transfer-ownership --organization-id "xxx-xxx-xxx" --user-id "auth0|xxx"
  qovery admin transfer-ownership --organization-id "xxx-xxx-xxx" --email "user@example.com"
  qovery admin transfer-ownership --organization-id "xxx-xxx-xxx" --email "user@example.com" --provider "github"
`,
		Run: func(cmd *cobra.Command, args []string) {
			transferOrganizationOwnership()
		},
	}
)

func init() {
	adminTransferOrganizationOwnership.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID (required)")
	adminTransferOrganizationOwnership.Flags().StringVarP(&newOwnerUserId, "user-id", "u", "", "New owner user ID")
	adminTransferOrganizationOwnership.Flags().StringVarP(&newOwnerEmail, "email", "e", "", "New owner email address")
	adminTransferOrganizationOwnership.Flags().StringVarP(&authProvider, "provider", "p", "", "Auth provider (auth0, github, gitlab, google, etc.) - required if multiple users have the same email")

	if err := adminTransferOrganizationOwnership.MarkFlagRequired("organization-id"); err != nil {
		utils.PrintlnError(fmt.Errorf("failed to mark organization-id flag as required: %w", err))
		os.Exit(1)
	}

	adminCmd.AddCommand(adminTransferOrganizationOwnership)
}

func transferOrganizationOwnership() {
	// Validate required fields
	if organizationId == "" {
		utils.PrintlnError(fmt.Errorf("organization ID is required"))
		os.Exit(1)
	}

	// Ensure either user ID or email is provided
	if newOwnerUserId == "" && newOwnerEmail == "" {
		utils.PrintlnError(fmt.Errorf("either --user-id or --email must be provided"))
		os.Exit(1)
	}

	if newOwnerUserId != "" && newOwnerEmail != "" {
		utils.PrintlnError(fmt.Errorf("only one of --user-id or --email should be provided, not both"))
		os.Exit(1)
	}

	// Get access token
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	// Get Qovery client
	client := utils.GetQoveryClient(tokenType, token)

	// If email is provided, find the user ID from organization members
	targetUserId := newOwnerUserId
	if newOwnerEmail != "" {
		utils.Println(fmt.Sprintf("🔍 Looking up user with email: %s", newOwnerEmail))

		members, res, err := client.MembersAPI.GetOrganizationMembers(context.Background(), organizationId).Execute()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("failed to list organization members: %w", err))
			if res != nil {
				utils.PrintlnError(fmt.Errorf("response status: %s", res.Status))
			}
			os.Exit(1)
		}

		// Find all members with matching email
		var matchingMembers []qovery.Member
		for _, member := range members.GetResults() {
			if member.Email == newOwnerEmail {
				matchingMembers = append(matchingMembers, member)
			}
		}

		if len(matchingMembers) == 0 {
			utils.PrintlnError(fmt.Errorf("no member found with email '%s' in organization %s", newOwnerEmail, organizationId))
			os.Exit(1)
		}

		// If multiple members found with the same email, check if provider is specified
		if len(matchingMembers) > 1 {
			if authProvider == "" {
				// Extract providers from user IDs
				var providers []string
				for _, member := range matchingMembers {
					// User ID format: "provider|id" (e.g., "auth0|123", "github|456")
					parts := strings.Split(member.Id, "|")
					if len(parts) >= 2 {
						providers = append(providers, parts[0])
					}
				}

				utils.PrintlnError(fmt.Errorf("multiple users found with email '%s'. Please specify --provider flag", newOwnerEmail))
				utils.PrintlnError(fmt.Errorf("available providers: %v", providers))
				os.Exit(1)
			}

			// Filter by provider
			var foundMember *qovery.Member
			for _, member := range matchingMembers {
				// User ID format: "provider|id"
				parts := strings.Split(member.Id, "|")
				if len(parts) >= 2 && strings.EqualFold(parts[0], authProvider) {
					foundMember = &member
					break
				}
			}

			if foundMember == nil {
				utils.PrintlnError(fmt.Errorf("no member found with email '%s' and provider '%s'", newOwnerEmail, authProvider))
				os.Exit(1)
			}

			targetUserId = foundMember.Id
			utils.Println(fmt.Sprintf("✅ Found user: %s (Provider: %s, ID: %s)", foundMember.Email, authProvider, targetUserId))
		} else {
			// Only one member found with this email
			targetUserId = matchingMembers[0].Id
			// Extract provider for display
			parts := strings.Split(targetUserId, "|")
			provider := "unknown"
			if len(parts) >= 2 {
				provider = parts[0]
			}
			utils.Println(fmt.Sprintf("✅ Found user: %s (Provider: %s, ID: %s)", matchingMembers[0].Email, provider, targetUserId))
		}
	}

	// Prepare transfer ownership request
	transferRequest := *qovery.NewTransferOwnershipRequest(targetUserId)

	// Execute transfer
	utils.Println(fmt.Sprintf("🔄 Transferring ownership to user %s...", targetUserId))
	res, err := client.MembersAPI.PostOrganizationTransferOwnership(context.Background(), organizationId).
		TransferOwnershipRequest(transferRequest).
		Execute()

	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to transfer ownership: %w", err))
		if res != nil {
			utils.PrintlnError(fmt.Errorf("response status: %s", res.Status))
		}
		os.Exit(1)
	}

	if res != nil && res.StatusCode >= 400 {
		utils.PrintlnError(fmt.Errorf("failed to transfer ownership with status: %s", res.Status))
		os.Exit(1)
	}

	if newOwnerEmail != "" {
		utils.Println(fmt.Sprintf("✅ Successfully transferred ownership of organization %s to %s", organizationId, newOwnerEmail))
	} else {
		utils.Println(fmt.Sprintf("✅ Successfully transferred ownership of organization %s to user %s", organizationId, targetUserId))
	}
}
