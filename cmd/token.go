package cmd

import (
	"context"
	"errors"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
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
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return "", err
	}

	roleId := qovery.NullableString{}
	roleId.Set(&tokenInformation.Role.ID)

	req := qovery.OrganizationApiTokenCreateRequest{
		Name:        tokenInformation.Name,
		Description: &tokenInformation.Description,
		Scope:       qovery.NullableOrganizationApiTokenScope{},
		RoleId:      roleId,
	}

	client := utils.GetQoveryClient(tokenType, token)
	createdToken, res, err := client.OrganizationApiTokenApi.CreateOrganizationApiToken(context.Background(), string(tokenInformation.Organization.ID)).OrganizationApiTokenCreateRequest(req).Execute()
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 400 {
		return "", errors.New("Received " + res.Status + " response while fetching environment. ")
	}

	return *createdToken.Token, nil
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
