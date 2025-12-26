package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	messageToEncrypt      string
	adminSecretEncryptCmd = &cobra.Command{
		Use:   "encrypt-secret",
		Short: "Encrypt a clear text message as a secret that can be used in core DB",
		Run: func(cmd *cobra.Command, args []string) {
			encryptSecret()
		},
	}
)

func init() {
	adminSecretEncryptCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID of which the secret need to be encrypted of")
	adminSecretEncryptCmd.Flags().StringVarP(&messageToEncrypt, "message", "m", "", "The message/value to encrypt")
	adminCmd.AddCommand(adminSecretEncryptCmd)
}

func encryptSecret() {
	var err error
	if organizationId == "" {
		utils.PrintlnInfo("organization-id is required")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if messageToEncrypt == "" {
		utils.PrintlnInfo("message is required")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	secret, err := callEncryptSecret(organizationId, messageToEncrypt)
	utils.CheckError(err)
	utils.PrintlnInfo(messageToEncrypt + " ==> " + secret)
}

func callEncryptSecret(organizationId string, secret string) (string, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// Build URL with proper escaping
	u, err := url.Parse(utils.GetAdminUrl())
	if err != nil {
		return "", fmt.Errorf("invalid admin URL: %w", err)
	}
	u.Path = path.Join(u.Path, "organization", organizationId, "secret")
	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(secret))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	// Create client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	// Check status code
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("failed to encrypt secret (status=%d)",
			res.StatusCode)
	}

	secretBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read body: %w", err)
	}
	return string(secretBytes), nil
}
