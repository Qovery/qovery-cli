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
	userEmail                string
	provider                 string
	adminEnableUserSignupCmd = &cobra.Command{
		Use:   "enable-user-connect",
		Short: "Allow a new user to connect after sign up",
		Long: `Allow a new user to connect after sign up with the specified email and authentication provider.

Example:
  qovery admin enable-user-connect --user-email "user@example.com"
  qovery admin enable-user-connect --user-email "user@example.com" --provider "github"
`,
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			// Check if required flags are provided
			if userEmail == "" {
				_ = cmd.Help()
				os.Exit(0)
			}

			enableUserSignup()
		},
	}
)

func init() {
	adminEnableUserSignupCmd.Flags().StringVarP(&userEmail, "user-email", "e", "", "User email address (required)")
	adminEnableUserSignupCmd.Flags().StringVarP(&provider, "provider", "p", "", "Authentication provider (github, gitlab, bitbucket, microsoft, google)")
	// Don't mark flags as required - we'll handle validation in the Run function
	adminCmd.AddCommand(adminEnableUserSignupCmd)
}

type EnableUserSignupRequest struct {
	UserEmail string `json:"user_email"`
	Provider  string `json:"provider,omitempty"`
}

// Provider enum

type Provider string

const (
	ProviderGithub    Provider = "GITHUB"
	ProviderGitlab    Provider = "GITLAB"
	ProviderBitbucket Provider = "BITBUCKET"
	ProviderMicrosoft Provider = "MICROSOFT"
	ProviderGoogle    Provider = "GOOGLE"
)

var validProviders = map[string]Provider{
	"github":    ProviderGithub,
	"gitlab":    ProviderGitlab,
	"bitbucket": ProviderBitbucket,
	"microsoft": ProviderMicrosoft,
	"google":    ProviderGoogle,
}

func (p Provider) String() string {
	return string(p)
}

func parseProvider(input string) (Provider, bool) {
	p, ok := validProviders[strings.ToLower(input)]
	return p, ok
}

func enableUserSignup() {
	// Validate required fields
	if userEmail == "" {
		utils.PrintlnError(fmt.Errorf("user email is required"))
		os.Exit(1)
	}

	var providerEnum Provider
	if provider != "" {
		var ok bool
		providerEnum, ok = parseProvider(provider)
		if !ok {
			// Show valid options in error
			var opts []string
			for k := range validProviders {
				opts = append(opts, k)
			}
			utils.PrintlnError(fmt.Errorf("invalid provider '%s'. Valid values are: %v", provider, opts))
			os.Exit(1)
		}
	}

	// Get access token
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	// Prepare request payload
	payload := EnableUserSignupRequest{
		UserEmail: userEmail,
	}
	if provider != "" {
		payload.Provider = providerEnum.String()
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("failed to marshal payload: %w", err))
		os.Exit(1)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/enableUserSignUp", utils.GetAdminUrl())
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
	if res.StatusCode == http.StatusOK {
		utils.Println("✅ User signup enabled successfully")
		if len(body) > 0 {
			utils.Println(fmt.Sprintf("Response: %s", string(body)))
		}
	} else {
		utils.PrintlnError(fmt.Errorf("❌ failed to enable user signup: %s - %s", res.Status, string(body)))
		os.Exit(1)
	}
}
