package cmd

import (
	"errors"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"strings"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generate an API token",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		utils.PrintlnInfo("Select organization")
		organization, err := utils.SelectOrganization()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		token, err := generateMachineToMachineAPIToken(organization)

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		utils.PrintlnInfo("---- Never share this authentication token and keep it secure ----")
		utils.PrintlnInfo(token)
		utils.PrintlnInfo("---- Never share this authentication token and keep it secure ----")
	},
}

func generateMachineToMachineAPIToken(organization *utils.Organization) (string, error) {
	token, err := utils.GetAccessToken()
	if err != nil {
		return "", err
	}

	// apiToken endpoint is not yet exposed in the OpenAPI spec at the moment. It's planned officially for Q3 2022
	req, err := http.NewRequest(http.MethodPost, string("https://api.qovery.com/organization/"+organization.ID+"/apiToken"), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	if res.StatusCode >= 400 {
		return "", errors.New("Received " + res.Status + " response while fetching environment. ")
	}

	result, _ := ioutil.ReadAll(res.Body)
	return string(result), nil
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
