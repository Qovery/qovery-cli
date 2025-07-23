package cmd

import (
	"context"
	"errors"
	_ "fmt"
	"os"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var rawFormat bool

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Print your application logs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		utils.ShowHelpIfNoArgs(cmd, args)
		getLogs()
	},
}

func getLogs() string {
	service, err := utils.CurrentService(true)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}
	org, _, _ := utils.CurrentOrganization(true)
	project, _, _ := utils.CurrentProject(true)
	env, _, _ := utils.CurrentEnvironment(true)

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	client := utils.GetQoveryClient(tokenType, token)
	e, res, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), string(env)).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	if res.StatusCode >= 400 {
		utils.PrintlnError(errors.New("Received " + res.Status + " response while fetching environment. "))
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	req := pkg.LogRequest{
		ServiceID:      service.ID,
		OrganizationID: org,
		ProjectID:      project,
		EnvironmentID:  env,
		ClusterID:      utils.Id(e.ClusterId),
		RawFormat:      rawFormat,
	}

	pkg.ExecLog(&req)

	//return logRows
	return ""
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVarP(&rawFormat, "raw", "r", false, "display logs in raw format (json)")
}
