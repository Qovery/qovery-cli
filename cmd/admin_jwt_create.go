package cmd

import (
	"bytes"
	"fmt"
	"github.com/go-jose/go-jose/v4/json"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminJwtCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a Jwt for a cluster",
		Run: func(cmd *cobra.Command, args []string) {
			createJwt()
		},
	}
)

func init() {
	adminJwtCreateCmd.Flags().StringVarP(&clusterId, "cluster", "c", "", "Cluster's id")

	adminJwtCmd.AddCommand(adminJwtCreateCmd)

}

func createJwt() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/clusters/%s/jwts", utils.GetAdminUrl(), clusterId)
	req, err := http.NewRequest(http.MethodPost, url,  bytes.NewBuffer([]byte("{  }")))
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

	jwt := struct {
		KeyId     string `json:"key_id"`
		ClusterId string `json:"cluster_id"`
		CreatedAt string `json:"created_at"`
	}{}

	if err := json.Unmarshal(body, &jwt); err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	format := "%s\t | %s\t | %s\t | %s\n"
	fmt.Fprintf(w, format, "", "cluster_id", "key_id", "created_at")
	fmt.Fprintf(w, format, fmt.Sprintf("%d", 1), jwt.ClusterId, jwt.KeyId, jwt.CreatedAt)
	w.Flush()
}
