package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminEnterpriseConnectionCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new enterprise connection",
		Run: func(cmd *cobra.Command, args []string) {
			createEnterpriseConnection()
		},
	}
)

func init() {
	adminEnterpriseConnectionCreateCmd.Flags().StringVarP(&enterpriseConnectionName, "connection-name", "c", "", "The connection name configured on Auth0 side for the target client")
	adminEnterpriseConnectionCreateCmd.Flags().StringVarP(&enterpriseConnectionOrganizationId, "organization-id", "o", "", "The organization of the target client")

	_ = adminEnterpriseConnectionCreateCmd.MarkFlagRequired("connection-name")
	_ = adminEnterpriseConnectionCreateCmd.MarkFlagRequired("organization-id")

	adminEnterpriseConnectionCmd.AddCommand(adminEnterpriseConnectionCreateCmd)
}

func createEnterpriseConnection() {
	// Retrieve access token for authorization
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		return
	}

	// Prepare payload with required fields
	payloadMap := map[string]string{
		"organization_id": enterpriseConnectionOrganizationId,
		"connection_name": enterpriseConnectionName,
	}
	payload, err := json.Marshal(payloadMap)
	if err != nil {
		utils.PrintlnError(err)
		return
	}

	// Build request
	url := fmt.Sprintf("%s/enterpriseconnection", utils.GetAdminUrl())
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
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

	// If not created, print the error message returned
	if res.StatusCode != http.StatusCreated {
		utils.PrintlnError(errors.New(string(body)))
		return
	}

	// Parse response as single EnterpriseConnection object
	var createdConnection EnterpriseConnection
	if err := json.Unmarshal(body, &createdConnection); err != nil {
		utils.PrintlnError(err)
		return
	}

	// Display created connection using PrintTable
	var data [][]string
	data = append(data, []string{
		createdConnection.OrganizationID,
		createdConnection.ConnectionName,
		createdConnection.DefaultRole,
	})

	err = utils.PrintTable([]string{"Organization ID", "Connection Name", "Default Role"}, data)
	if err != nil {
		utils.PrintlnError(err)
		return
	}
}
