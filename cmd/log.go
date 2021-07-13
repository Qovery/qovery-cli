package cmd

import (
	"errors"
	_ "fmt"
	"github.com/olekukonko/tablewriter"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"os"
	"time"
)

var follow bool

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Displays application logs",
	Run: func(cmd *cobra.Command, args []string) {
		var logs = getLogs()

		table := setupTable(true)
		table.AppendBulk(logs)
		table.Render()

		if len(logs) <= 0 {
			utils.PrintlnInfo("No logs found")
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
	token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	application, _, err := utils.CurrentApplication()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	logs, res, err := client.ApplicationLogsApi.ListApplicationLog(auth, string(application)).Execute()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	if res.StatusCode >= 400 {
		utils.PrintlnError(errors.New("Received " + res.Status + " response while listing organizations"))
	}

	var logRows = make([][]string, 0)

	for _, log := range logs.GetResults() {
		logRows = append(logRows, []string{log.CreatedAt.Format(time.StampMicro), log.Message})
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
	table.SetAutoWrapText(false)
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
