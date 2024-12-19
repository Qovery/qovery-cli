package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

type listLogResponse struct {
	Filename     string `json:"filename"`
	LastModified string `json:"last_modified"`
}

var (
	adminDemoListLogsCmd = &cobra.Command{
		Use:   "list-logs",
		Short: "list error logs from the command demo up",
		Run: func(cmd *cobra.Command, args []string) {
			utils.Capture(cmd)

			tokenType, token, err := utils.GetAccessToken()
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}

			url := fmt.Sprintf("%s/demoDebugLog", utils.GetAdminUrl())
			req, _ := http.NewRequest(http.MethodGet, url, bytes.NewReader([]byte{}))
			query := req.URL.Query()
			orgaId, _ := cmd.Flags().GetString("organizationId")
			if orgaId != "" {
				query.Add("organization", orgaId)
			}
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

			var responseObject []listLogResponse
			_ = json.Unmarshal(body, &responseObject)

			var rows [][]string
			for _, row := range responseObject {
				rows = append(rows, []string{row.LastModified, row.Filename})
			}
			_ = utils.PrintTable([]string{"date", "filename"}, rows)
		},
	}
)

func init() {
	adminDemoListLogsCmd.Flags().StringP("organizationId", "o", "", "Organization to filter on for listing")
	adminDemoCmd.AddCommand(adminDemoListLogsCmd)
}
