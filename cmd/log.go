package cmd

import (
	"context"
	"errors"
	_ "fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var follow bool

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Print your application logs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		var logs = getLogs()

		table := setupTable(true)
		table.AppendBulk(logs)
		table.Render()

		if len(logs) <= 0 {
			utils.PrintlnInfo("No logs found. ")
			os.Exit(0)
		}

		var lastRenderedLogs = logs

		for follow {
			table := setupTable(false)

			lastLogDateString := lastRenderedLogs[len(lastRenderedLogs)-1][0]
			lastLogDate, _ := time.Parse(time.StampMicro, lastLogDateString)
			var newLogs = getLogs()

			if len(newLogs) > 0 {
				for _, newLog := range newLogs {
					newLogDate, _ := time.Parse(time.StampMicro, newLog[0])
					if lastLogDate.Before(newLogDate) {
						table.Append(newLog)
					}
				}
				table.Render()
				lastRenderedLogs = newLogs
			}

			time.Sleep(time.Second * 5)
		}
	},
}

func getLogs() [][]string {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	service, err := utils.CurrentService()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	client := utils.GetQoveryClient(tokenType, token)

	var logRows = make([][]string, 0)
	switch service.Type {
	case utils.ApplicationType:
		logs, res, err := client.ApplicationLogsApi.ListApplicationLog(context.Background(), string(service.ID)).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
		if res.StatusCode >= 400 {
			utils.PrintlnError(errors.New("Received " + res.Status + " response while getting application logs "))
		}

		for _, log := range logs.GetResults() {
			logRows = append(logRows, []string{log.CreatedAt.Format(time.StampMicro), log.Message})
		}
	case utils.ContainerType:
		logs, res, err := client.ContainerLogsApi.ListContainerLog(context.Background(), string(service.ID)).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
		if res.StatusCode >= 400 {
			utils.PrintlnError(errors.New("Received " + res.Status + " response while getting container logs"))
		}

		for _, log := range logs.GetResults() {
			logRows = append(logRows, []string{log.CreatedAt.Format(time.StampMicro), log.Message})
		}
	}

	return logRows
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
	logCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow application logs")
}
