package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var environmentUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an environment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, projectId, err := getOrganizationProjectContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), projectId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		env := utils.FindByEnvironmentName(environments.GetResults(), environmentName)

		if env == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", environmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		m := getEnvironmentType(string(env.Mode))
		req := qovery.EnvironmentEditRequest{
			Name: &env.Name,
			Mode: &m,
		}

		if newEnvironmentName != "" {
			req.Name = &newEnvironmentName
		}

		if environmentType != "" {
			m = getEnvironmentType(environmentType)
			req.Mode = &m
		}

		_, _, err = client.EnvironmentMainCallsAPI.EditEnvironment(context.Background(), env.Id).EnvironmentEditRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println("Environment is updated!")
	},
}

func getEnvironmentType(environmentType string) qovery.CreateEnvironmentModeEnum {
	switch strings.ToUpper(environmentType) {
	case "DEVELOPMENT":
		return qovery.CREATEENVIRONMENTMODEENUM_DEVELOPMENT
	case "PRODUCTION":
		return qovery.CREATEENVIRONMENTMODEENUM_PRODUCTION
	case "STAGING":
		return qovery.CREATEENVIRONMENTMODEENUM_STAGING
	}

	return qovery.CREATEENVIRONMENTMODEENUM_DEVELOPMENT
}

func init() {
	environmentCmd.AddCommand(environmentUpdateCmd)
	environmentUpdateCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	environmentUpdateCmd.Flags().StringVarP(&projectName, "project", "p", "", "Project Name")
	environmentUpdateCmd.Flags().StringVarP(&environmentName, "environment", "e", "", "Environment Name")
	environmentUpdateCmd.Flags().StringVarP(&newEnvironmentName, "name", "", "", "New Environment Name")
	environmentUpdateCmd.Flags().StringVarP(&environmentType, "type", "", "", "Change Environment Type (DEVELOPMENT|STAGING|PRODUCTION)")
}
