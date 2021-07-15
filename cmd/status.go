package cmd

import (
	"errors"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"os"
	"time"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the status of your application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
		application, name, err := utils.CurrentApplication()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

		status, res, err := client.ApplicationMainCallsApi.GetApplicationStatus(auth, string(application)).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
		if res.StatusCode >= 400 {
			utils.PrintlnError(errors.New("Received " + res.Status + " response while listing organizations. "))
		}

		fmt.Printf("%v\n", time.Now().Format(time.RFC822))
		err = pterm.DefaultTable.WithData(pterm.TableData{{"Application", string(name)}, {"Status", status.State}}).Render()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
