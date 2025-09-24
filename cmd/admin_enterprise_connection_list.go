package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminEnterpriseConnectionListCmd = &cobra.Command{
		Use:   "list",
		Short: "List enterprise connections by connection name",
		Run: func(cmd *cobra.Command, args []string) {
			listEnterpriseConnections()
		},
	}
)

func init() {
	adminEnterpriseConnectionListCmd.Flags().StringVarP(&enterpriseConnectionName, "connection-name", "c", "", "The connection name configured on Auth0 side for the target client")
	_ = adminEnterpriseConnectionListCmd.MarkFlagRequired("connection-name")

	adminEnterpriseConnectionCmd.AddCommand(adminEnterpriseConnectionListCmd)
}

func listEnterpriseConnections() {
	// Retrieve access token for authorization
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		return
	}

	// Build URL
	cn := url.PathEscape(enterpriseConnectionName)

	// Real URL example:
	url := fmt.Sprintf("%s/enterpriseconnection/%s", utils.GetAdminUrl(), cn)

	// Prepare request
	req, err := http.NewRequest(http.MethodGet, url, nil)
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

	// If not OK, print the error message returned
	if res.StatusCode != http.StatusOK {
		utils.PrintlnError(fmt.Errorf(string(body)))
		return
	}

	// Parse response wrapped in { "results": [...] }
	wrapped := struct {
		Results []EnterpriseConnection `json:"results"`
	}{}

	if err := json.Unmarshal(body, &wrapped); err != nil {
		utils.PrintlnError(err)
		return
	}

	list := wrapped.Results

	// Display results using PrintTable
	var data [][]string

	for _, ec := range list {
		data = append(data, []string{
			ec.OrganizationID,
			ec.ConnectionName,
			ec.DefaultRole,
		})
	}

	err = utils.PrintTable([]string{"Organization ID", "Connection Name", "Default Role"}, data)
	if err != nil {
		utils.PrintlnError(err)
		return
	}
}
