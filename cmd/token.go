package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"strings"
)

type TokenCreationResponseDto struct {
	Token string
}

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generate an API token",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		utils.PrintlnInfo("Select organization")
		tokenInformation, err := utils.SelectTokenInformation()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		token, err := generateMachineToMachineAPIToken(tokenInformation)

		if err != nil {
			utils.PrintlnError(err)
			return
		}

		utils.PrintlnInfo("---- Never share this authentication token and keep it secure ----")
		utils.PrintlnInfo(token)
		utils.PrintlnInfo("---- Never share this authentication token and keep it secure ----")
	},
}

func generateMachineToMachineAPIToken(tokenInformation *utils.TokenInformation) (string, error) {
	token, err := utils.GetAccessToken()
	if err != nil {
		return "", err
	}

	requestBody, err := json.Marshal(map[string]string{
		"name":        tokenInformation.Name,
		"description": tokenInformation.Description,
		"scope":       "ADMIN",
	})

	if err != nil {
		return "", err
	}

	// apiToken endpoint is not yet exposed in the OpenAPI spec at the moment. It's planned officially for Q3 2022
	req, err := http.NewRequest(
		http.MethodPost,
		string("https://api.qovery.com/organization/"+tokenInformation.Organization.ID+"/apiToken"),
		bytes.NewBuffer(requestBody),
	)
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

	jsonResponse, _ := io.ReadAll(res.Body)
	var tokenCreationResponseDto TokenCreationResponseDto

	err = json.Unmarshal(jsonResponse, &tokenCreationResponseDto)
	if err != nil {
		return "", err
	}

	return tokenCreationResponseDto.Token, nil
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
