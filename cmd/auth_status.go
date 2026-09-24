package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status without exposing the access token",
	Long: `Show whether the CLI is currently authenticated, as whom, which organization
is selected, and when the session expires.

This command never prints the access token or any other secret value, regardless
of flags. Use it as the safe way to check authentication from scripts, CI, and
automation, instead of parsing the output of 'qovery auth token':

  qovery auth status >/dev/null 2>&1 && echo authenticated || echo "not authenticated"

Unlike most other commands, this always makes a live call to the Qovery API to
confirm the token is currently accepted — a token that is present and well-formed
but has been revoked or expired server-side is reported as not authenticated,
whether it came from a browser login or from QOVERY_CLI_ACCESS_TOKEN / Q_CLI_ACCESS_TOKEN.

Exit code is 0 when authenticated, 1 otherwise.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken(true)
		if err != nil {
			printAuthStatus(authStatusOutput{
				Authenticated: false,
				APIURL:        utils.GetAPIBaseURL(),
			})
			os.Exit(1)
		}

		// GetAccessToken only verifies validity server-side for browser/device-flow
		// (context-stored) sessions. A token supplied via QOVERY_CLI_ACCESS_TOKEN or
		// Q_CLI_ACCESS_TOKEN is returned as-is with no server round trip, so it can look
		// well-formed while already being revoked or expired. Always re-verify here,
		// regardless of where the token came from, so "authenticated" means "the server
		// currently accepts this token" rather than "a token-shaped string exists".
		client := utils.GetQoveryClient(tokenType, token)
		if _, _, verifyErr := client.OrganizationMainCallsAPI.ListOrganization(context.Background()).Execute(); verifyErr != nil {
			printAuthStatus(authStatusOutput{
				Authenticated: false,
				APIURL:        utils.GetAPIBaseURL(),
			})
			os.Exit(1)
		}

		output := authStatusOutput{
			Authenticated: true,
			TokenType:     string(tokenType),
			APIURL:        utils.GetAPIBaseURL(),
		}

		// Best-effort: context is only populated for browser/device-flow (Bearer) logins,
		// not for a raw API token passed via QOVERY_CLI_ACCESS_TOKEN / Q_CLI_ACCESS_TOKEN.
		if ctx, ctxErr := utils.GetCurrentContext(); ctxErr == nil {
			if !ctx.AccessTokenExpiration.IsZero() {
				output.ExpiresAt = ctx.AccessTokenExpiration.UTC().Format("2006-01-02T15:04:05Z")
			}
			output.OrganizationId = string(ctx.OrganizationId)
			output.OrganizationName = string(ctx.OrganizationName)
			output.User = string(ctx.User)
		}

		printAuthStatus(output)
	},
}

type authStatusOutput struct {
	Authenticated    bool   `json:"authenticated"`
	TokenType        string `json:"token_type,omitempty"`
	ExpiresAt        string `json:"expires_at,omitempty"`
	OrganizationId   string `json:"organization_id,omitempty"`
	OrganizationName string `json:"organization_name,omitempty"`
	User             string `json:"user,omitempty"`
	APIURL           string `json:"api_url"`
}

func printAuthStatus(output authStatusOutput) {
	if jsonFlag {
		jsonBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		utils.Println(string(jsonBytes))
		return
	}

	if !output.Authenticated {
		utils.Println("Not authenticated. Run 'qovery auth' to log in.")
		return
	}

	utils.Println("Authenticated:  yes")
	utils.Println("Token type:     " + output.TokenType)
	if output.ExpiresAt != "" {
		utils.Println("Expires at:     " + output.ExpiresAt)
	}
	if output.OrganizationName != "" {
		utils.Println("Organization:   " + output.OrganizationName + " (" + output.OrganizationId + ")")
	}
	if output.User != "" {
		utils.Println("User:           " + output.User)
	}
	utils.Println("API URL:        " + output.APIURL)
}

func init() {
	authCmd.AddCommand(authStatusCmd)
	authStatusCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")
}
