package cmd

import (
	"context"
	"errors"
	_ "fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var rawFormat bool

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Print your application logs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		getLogs()
	},
}

func getLogs() string {
	service, err := utils.CurrentService()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}
	orga, _, _ := utils.CurrentOrganization()
	project, _, _ := utils.CurrentProject()
	env, _, _ := utils.CurrentEnvironment()

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	client := utils.GetQoveryClient(tokenType, token)
	e, res, err := client.EnvironmentMainCallsApi.GetEnvironment(context.Background(), string(env)).Execute()
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
		OrganizationID: orga,
		ProjectID:      project,
		EnvironmentID:  env,
		ClusterID:      utils.Id(e.ClusterId),
		RawFormat:      rawFormat,
	}

	pkg.ExecLog(&req)

	//return logRows
	return ""
}

func setupTable(header bool) *tablewriter.Table {
	table := tablewriter.NewWriter(os.Stdout)

	if header {
		table.SetHeader([]string{"TIME", "MESSAGE"})
	}

	table.SetBorder(false)
	table.SetHeaderLine(false)
	table.SetColumnSeparator("")
	table.SetAutoWrapText(true)
	table.SetRowLine(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetColWidth(160)
	table.SetBorders(tablewriter.Border{
		Left:   false,
		Right:  false,
		Top:    false,
		Bottom: false,
	})

	return table
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVarP(&rawFormat, "raw", "r", false, "display logs in raw format (json)")
}
