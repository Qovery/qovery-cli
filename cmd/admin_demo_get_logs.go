package cmd

import (
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

type ListLogResponse struct {
	Filename     string `json:"filename"`
	LastModified string `json:"last_modified"`
}

var (
	adminDemoGetLogsCmd = &cobra.Command{
		Use:   "get-log",
		Short: "retrieve a specific log",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			if len(args) == 0 {
				log.Fatal("You must specify a log filename as argument")
				os.Exit(0)
			}

			tokenType, token, err := utils.GetAccessToken()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}

			url := fmt.Sprintf("%s/demoDebugLog", utils.GetAdminUrl())
			req, _ := http.NewRequest(http.MethodGet, url, bytes.NewReader([]byte{}))
			query := req.URL.Query()
			query.Add("filename", args[0])
			req.URL.RawQuery = query.Encode()
			req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))

			response, err := http.DefaultClient.Do(req)
			if err != nil {
				utils.PrintlnError(fmt.Errorf("error uploading debug logs: %s", err))
				return
			}

			body, _ := io.ReadAll(response.Body)
			if response.StatusCode != http.StatusOK {
				utils.PrintlnError(fmt.Errorf("error uploading debug logs: %s %s", response.Status, body))
				return
			}

			_ = os.WriteFile(args[0], body, 0640)
			log.Info("file written to ", args[0])
		},
	}
)

func init() {
	adminDemoCmd.AddCommand(adminDemoGetLogsCmd)
}
