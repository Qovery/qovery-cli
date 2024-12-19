package cmd

import (
	"bytes"
	"fmt"
	"github.com/go-jose/go-jose/v4/json"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminJwtForQoveryUsageCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a Jwt for Qovery usage",
		Run: func(cmd *cobra.Command, args []string) {
			createJwtForQoveryUsage()
		},
	}
)

func init() {
	adminJwtForQoveryUsageCreateCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster's id")
	adminJwtForQoveryUsageCreateCmd.Flags().StringVarP(&organizationId, "organization-id", "", "", "Organization's id")
	adminJwtForQoveryUsageCreateCmd.Flags().StringVarP(&rootDns, "root-dns", "", "", "root dns")
	adminJwtForQoveryUsageCreateCmd.Flags().StringVarP(&additionalClaims, "additional-claims", "", "{}", "Additional claims in JSON format (e.g., '{\"key1\":\"value1\",\"key2\":\"value2\"}')")
	adminJwtForQoveryUsageCreateCmd.Flags().StringVarP(&description, "description", "d", "", "Description of the JWT")

	adminJwtForQoveryUsageCmd.AddCommand(adminJwtForQoveryUsageCreateCmd)
}

func createJwtForQoveryUsage() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	var claimsMap map[string]string
	err = json.Unmarshal([]byte(additionalClaims), &claimsMap)
	if err != nil {
		fmt.Printf("Error when parsing additional-claims : %v\n", err)
		return
	}

	type Payload struct {
		OrganizationId   string            `json:"organization_id"`
		ClusterId        string            `json:"cluster_id"`
		RootDns          string            `json:"root_dns"`
		AdditionalClaims map[string]string `json:"additional_claims"`
		Description      string            `json:"description"`
	}

	var payload, _ = json.Marshal(Payload{
		ClusterId:        clusterId,
		OrganizationId:   organizationId,
		RootDns:          rootDns,
		AdditionalClaims: claimsMap,
		Description:      description,
	})

	url := fmt.Sprintf("%s/jwts", utils.GetAdminUrl())
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
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

	jwtForQoveryUsage := struct {
		KeyId       string `json:"key_id"`
		Description string `json:"description"`
		Jwt         string `json:"decrypted_jwt"`
		CreatedAt   string `json:"created_at"`
	}{}

	if err := json.Unmarshal(body, &jwtForQoveryUsage); err != nil {
		log.Fatal(err)
	}
	_, jwtPayload, err := DecodeJWT(jwtForQoveryUsage.Jwt)
	if err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)

	_, _ = fmt.Fprintln(w, "Field\t | Value")
	_, _ = fmt.Fprintln(w, "------\t | ------")

	_, _ = fmt.Fprintf(w, "key_id\t | %s\n", jwtForQoveryUsage.KeyId)
	_, _ = fmt.Fprintf(w, "description\t | %s\n", jwtForQoveryUsage.Description)
	_, _ = fmt.Fprintf(w, "jwt payload\t | %s\n", jwtPayload)
	_, _ = fmt.Fprintf(w, "jwt\t | %s\n", jwtForQoveryUsage.Jwt)
	_, _ = fmt.Fprintf(w, "created_at\t | %s\n", jwtForQoveryUsage.CreatedAt)
	_ = w.Flush()
}
