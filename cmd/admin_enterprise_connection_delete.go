package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminEnterpriseConnectionDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete an enterprise connection",
		Run: func(cmd *cobra.Command, args []string) {
			deleteEnterpriseConnection()
		},
	}
)

func init() {
	adminEnterpriseConnectionDeleteCmd.Flags().StringVarP(&enterpriseConnectionName, "connection-name", "c", "", "The connection name configured on Auth0 side for the target client")
	adminEnterpriseConnectionDeleteCmd.Flags().StringVarP(&enterpriseConnectionOrganizationId, "organization-id", "o", "", "The organization of the target client")

	_ = adminEnterpriseConnectionDeleteCmd.MarkFlagRequired("connection-name")
	_ = adminEnterpriseConnectionDeleteCmd.MarkFlagRequired("organization-id")

	adminEnterpriseConnectionCmd.AddCommand(adminEnterpriseConnectionDeleteCmd)
}

func deleteEnterpriseConnection() {
	// Retrieve access token for authorization
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		return
	}

	// Build URL
	cn := url.PathEscape(enterpriseConnectionName)
	oid := url.QueryEscape(enterpriseConnectionOrganizationId)

	// Real URL example:
	url := fmt.Sprintf("%s/admin/enterpriseconnection/%s?organization_id=%s", utils.GetAdminUrl(), cn, oid)

	// Prepare request
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()

	// Read response
	body, _ := io.ReadAll(res.Body)

	// If not accepted, print the error message returned
	if res.StatusCode != http.StatusAccepted {
		utils.PrintlnError(fmt.Errorf(string(body)))
		return
	}
}
