package cmd

import (
	"context"
	"errors"
	_ "fmt"
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var rawFormat bool
var logDownload bool
var logOutputFile string

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Display service logs",
	Long: `Display logs for any service (application, container, cronjob, lifecycle job) in real-time or download them to a file.

Examples:
  # Stream logs in real-time
  qovery log

  # Display logs in raw JSON format
  qovery log --raw

  # Download logs to a file
  qovery log --download --output-file ./service-logs.txt

  # Works with any service type (application, container, cronjob, lifecycle job, etc.)
  qovery context set service my-cronjob
  qovery log --download
`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
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
		Download:       logDownload,
		OutputFile:     logOutputFile,
	}

	if logDownload {
		if err := pkg.DownloadLogs(&req); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		utils.PrintlnInfo("Logs downloaded successfully to " + logOutputFile)
	} else {
		pkg.ExecLog(&req)
	}

	//return logRows
	return ""
}

func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().BoolVarP(&rawFormat, "raw", "r", false, "display logs in raw format (json)")
	logCmd.Flags().BoolVarP(&logDownload, "download", "d", false, "download logs to a file instead of streaming")
	logCmd.Flags().StringVarP(&logOutputFile, "output-file", "o", "service-logs.txt", "output file path when downloading logs")
}
