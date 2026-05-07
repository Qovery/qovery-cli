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
var authTokenPrintFlag bool

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Output the current valid access token",
	Long: `Output the current valid access token (refreshing it if expired).

This command provides a valid access token that can be used to make direct API calls 
to the Qovery API. The token is automatically refreshed if it has expired.

For security reasons, the token is not printed by default. You must explicitly 
use --print or --json to output the token value.

Examples:
  # Print the raw token value
  qovery auth token --print

  # Use directly in a curl command
  curl -H "Authorization: Bearer $(qovery auth token --print)" https://api.qovery.com/organization

  # Print the full Authorization header value
  qovery auth token --print --authorization-header

  # Get structured JSON output with token, type, expiration, and API URL
  qovery auth token --json

  # Get JSON with the authorization header pre-formatted
  qovery auth token --json --authorization-header`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		// If neither --print nor --json is set, show help and available flags
		if !authTokenPrintFlag && !authTokenJsonFlag {
			_ = cmd.Help()
			return
		}

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

		// --print: just the raw token value
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
	authTokenCmd.Flags().BoolVar(&authTokenPrintFlag, "print", false, "Print the raw access token value to stdout")
	authTokenCmd.Flags().BoolVar(&authTokenJsonFlag, "json", false, "Output as JSON with token, type, expiration, and API URL")
	authTokenCmd.Flags().BoolVar(&authTokenAuthorizationHeaderFlag, "authorization-header", false, "Output the full Authorization header value (e.g. 'Bearer eyJ...')")
}
