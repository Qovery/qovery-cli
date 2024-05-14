package cmd

import (
	"context"
	"errors"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the status of your application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
		service, err := utils.CurrentService(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		client := utils.GetQoveryClient(tokenType, token)

		switch service.Type {
		case utils.ApplicationType:
			status, res, err := client.ApplicationMainCallsAPI.GetApplicationStatus(context.Background(), string(service.ID)).Execute()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(0)
			}
			if res.StatusCode >= 400 {
				utils.PrintlnError(errors.New("Received " + res.Status + " response while listing organizations. "))
			}

			err = pterm.DefaultTable.WithData(pterm.TableData{{"Application", "Status"}, {string(service.Name), string(status.State)}}).Render()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(0)
			}
		case utils.ContainerType:
			status, res, err := client.ContainerMainCallsAPI.GetContainerStatus(context.Background(), string(service.ID)).Execute()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(0)
			}
			if res.StatusCode >= 400 {
				utils.PrintlnError(errors.New("Received " + res.Status + " response while listing organizations. "))
			}

			err = pterm.DefaultTable.WithData(pterm.TableData{{"Container", "Status"}, {string(service.Name), string(status.State)}}).Render()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(0)
			}
		}

	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
