package cmd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net/http"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

var (
	adminJwtDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a Jwt",
		Run: func(cmd *cobra.Command, args []string) {
			deleteJwt()
		},
	}
)

func init() {
	adminJwtDeleteCmd.Flags().StringVarP(&jwtKid, "kid", "", "", "Cluster's id")

	adminJwtCmd.AddCommand(adminJwtDeleteCmd)

}

func deleteJwt() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/clusters/jwts/%s", utils.GetAdminUrl(), jwtKid)
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if res.StatusCode != http.StatusNoContent {
		utils.PrintlnError(fmt.Errorf("error: %s", res.Status))
		return
	}

	if err != nil {
		log.Fatal(err)
	}
}
