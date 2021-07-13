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
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		application, name, err := utils.CurrentApplication()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
		client := qovery.NewAPIClient(qovery.NewConfiguration())

		status, res, err := client.ApplicationMainCallsApi.GetApplicationStatus(auth, string(application)).Execute()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		if res.StatusCode >= 400 {
			utils.PrintlnError(errors.New("Received " + res.Status + " response while listing organizations"))
		}

		fmt.Printf("%v\n", time.Now().Format(time.RFC822))
		err = pterm.DefaultTable.WithData(pterm.TableData{{"Application", string(name)}, {"Status", status.State}}).Render()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
