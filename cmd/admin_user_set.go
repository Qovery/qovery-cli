package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"net/url"
	"qovery-cli/io"
	"strings"
)

type payload struct {
	UserMetadata userMetadata `json:"user_metadata"`
}

type userMetadata struct {
	Rsub string `json:"rsub"`
}

var adminUserSetCmd = &cobra.Command{
	Use:  "set",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		authorizationToken, adminToken := GetTokens()
		payload := prepareUserMetadataPayload(args[0])
		subClaim := getSubClaim(authorizationToken)

		request, err := http.NewRequest("PATCH", "https://auth.qovery.com/api/v2/users/"+url.QueryEscape(subClaim), payload)
		if err != nil {
			log.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(adminToken))
		request.Header.Set("Content-Type", "application/json")

		res, err := http.DefaultClient.Do(request)
		if err != nil {
			log.Fatal(err)
		}

		if res.StatusCode != 200 {
			log.Printf("Could not set replacement user. ")
		} else {
			log.Printf("OK!")
			io.DoRequestUserToAuthenticate(false)
		}
	},
}

func init() {
	adminUserCmd.AddCommand(adminUserSetCmd)
}
