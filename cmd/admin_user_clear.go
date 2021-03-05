package cmd

import (
	"github.com/Qovery/qovery-cli/io"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type clearUserMetadataPayload struct {
	UserMetadata emptyUserMetadata `json:"user_metadata"`
}

type emptyUserMetadata struct{}

var adminUserClearCmd = &cobra.Command{
	Use:  "clear",
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		authorizationToken, adminToken := getTokens()
		payload := prepareEmptyUserMetadataPayload()
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
			log.Printf("Could not clear replacement user. ")
		} else {
			log.Printf("OK!")
			io.DoRequestUserToAuthenticate(false)
		}
	},
}

func init() {
	adminUserCmd.AddCommand(adminUserClearCmd)
}
