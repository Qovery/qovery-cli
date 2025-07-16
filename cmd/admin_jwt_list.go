package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminJwtListCmd = &cobra.Command{
		Use:   "list",
		Short: "List Jwt of a cluster",
		Run: func(cmd *cobra.Command, args []string) {
			listJwts()
		},
	}
)

func init() {
	adminJwtListCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")

	adminJwtCmd.AddCommand(adminJwtListCmd)

}

func listJwts() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/clusters/%s/jwts", utils.GetAdminUrl(), clusterId)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		utils.PrintlnError(fmt.Errorf("error uploading debug logs: %s %s", res.Status, body))
		return
	}

	resp := struct {
		Results []struct {
			ClusterId string `json:"cluster_id"`
			KeyId     string `json:"key_id"`
			CreatedAt string `json:"created_at"`
		} `json:"results"`
	}{}

	if err := json.Unmarshal(body, &resp); err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	format := "%s\t | %s\t | %s\t | %s\n"
	if _, err := fmt.Fprintf(w, format, "", "cluster_id", "key_id", "created_at"); err != nil {
		log.Fatal(err)
	}
	for idx, jwt := range resp.Results {
		if _, err := fmt.Fprintf(w, format, fmt.Sprintf("%d", idx+1), jwt.ClusterId, jwt.KeyId, jwt.CreatedAt); err != nil {
			log.Fatal(err)
		}
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}
}
