package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminJwtForQoveryUsageListCmd = &cobra.Command{
		Use:   "list",
		Short: "List Jwt for Qovery usage",
		Run: func(cmd *cobra.Command, args []string) {
			listJwtsForQoveryUsage()
		},
	}
)

func init() {
	adminJwtForQoveryUsageListCmd.Flags()

	adminJwtForQoveryUsageCmd.AddCommand(adminJwtForQoveryUsageListCmd)

}

func listJwtsForQoveryUsage() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/jwts", utils.GetAdminUrl())
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		utils.PrintlnError(fmt.Errorf("error uploading debug logs: %s %s", res.Status, body))
		return
	}

	resp := struct {
		Results []struct {
			KeyId       string `json:"key_id"`
			Description string `json:"description"`
			Jwt         string `json:"decrypted_jwt"`
			CreatedAt   string `json:"created_at"`
		} `json:"results"`
	}{}
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	for idx, jwtForQoveryUsage := range resp.Results {
		_, jwtPayload, err := DecodeJWT(jwtForQoveryUsage.Jwt)
		if err != nil {
			log.Fatal(err)
		}
		_, _ = fmt.Fprintln(w, "\t")
		_, _ = fmt.Fprintln(w, "Field\t | Value")
		_, _ = fmt.Fprintln(w, "------\t | ------")

		_, _ = fmt.Fprintf(w, "index\t | %s\n", fmt.Sprintf("%d", idx+1))
		_, _ = fmt.Fprintf(w, "key_id\t | %s\n", jwtForQoveryUsage.KeyId)
		_, _ = fmt.Fprintf(w, "description\t | %s\n", jwtForQoveryUsage.Description)
		_, _ = fmt.Fprintf(w, "jwt payload\t | %s\n", jwtPayload)
		_, _ = fmt.Fprintf(w, "jwt\t | %s\n", jwtForQoveryUsage.Jwt)
		_, _ = fmt.Fprintf(w, "created_at\t | %s\n", jwtForQoveryUsage.CreatedAt)
	}
	_ = w.Flush()
}

func DecodeJWT(tokenString string) (string, string, error) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return "", "", fmt.Errorf("failed to parse token: %w", err)
	}

	headerJSON, err := json.Marshal(token.Header)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal header: %w", err)
	}

	claimsJSON, err := json.Marshal(token.Claims)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal claims: %w", err)
	}

	return string(headerJSON), string(claimsJSON), nil
}
