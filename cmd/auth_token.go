package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var authTokenJsonFlag bool
var authTokenAuthorizationHeaderFlag bool

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the current valid access token",
	Long: `Print the current valid access token (refreshing it if expired).

This command outputs a valid access token that can be used to make direct API calls 
to the Qovery API. The token is automatically refreshed if it has expired.

By default, only the raw token value is printed to stdout (no newline formatting, 
no extra text), making it safe for shell substitution.

Examples:
  # Get the raw token value (default)
  qovery auth token

  # Use directly in a curl command
  curl -H "Authorization: Bearer $(qovery auth token)" https://api.qovery.com/organization

  # Get the full Authorization header value
  qovery auth token --authorization-header

  # Get structured JSON output with token, type, expiration, and API URL
  qovery auth token --json

  # Get JSON with the authorization header pre-formatted
  qovery auth token --json --authorization-header`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error: "+err.Error())
			os.Exit(1)
		}

		if authTokenJsonFlag {
			printTokenAsJSON(tokenType, token)
			return
		}

		if authTokenAuthorizationHeaderFlag {
			fmt.Print(utils.GetAuthorizationHeaderValue(tokenType, token))
			return
		}

		// Default: just the raw token value
		fmt.Print(string(token))
	},
}

type authTokenJSONOutput struct {
	AccessToken         string `json:"access_token,omitempty"`
	TokenType           string `json:"token_type,omitempty"`
	AuthorizationHeader string `json:"authorization_header,omitempty"`
	ExpiresAt           string `json:"expires_at,omitempty"`
	APIURL              string `json:"api_url"`
}

func printTokenAsJSON(tokenType utils.AccessTokenType, token utils.AccessToken) {
	output := authTokenJSONOutput{
		APIURL: utils.GetAPIBaseURL(),
	}

	if authTokenAuthorizationHeaderFlag {
		output.AuthorizationHeader = utils.GetAuthorizationHeaderValue(tokenType, token)
	} else {
		output.AccessToken = string(token)
		output.TokenType = string(tokenType)
	}

	// Try to get expiration from context (only available for Bearer tokens from context.json)
	if tokenType == "Bearer" {
		if ctx, err := utils.GetCurrentContext(); err == nil && !ctx.AccessTokenExpiration.IsZero() {
			output.ExpiresAt = ctx.AccessTokenExpiration.UTC().Format("2006-01-02T15:04:05Z")
		}
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: failed to marshal JSON output: "+err.Error())
		os.Exit(1)
	}
	fmt.Println(string(jsonBytes))
}

func init() {
	authCmd.AddCommand(authTokenCmd)
	authTokenCmd.Flags().BoolVar(&authTokenJsonFlag, "json", false, "Output as JSON with token, type, expiration, and API URL")
	authTokenCmd.Flags().BoolVar(&authTokenAuthorizationHeaderFlag, "authorization-header", false, "Output the full Authorization header value (e.g. 'Bearer eyJ...')")
}
