package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/api"
	"strconv"
)

const (
	httpAuthPort   = 10999
	oAuthQoveryUrl = "https://auth.qovery.com/login?client=%s&protocol=oauth2&response_type=%s&audience=%s&redirect_uri=%s"
)

var (
	oAuthUrlParamValueClient    = "MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0"
	oAuthUrlParamValueAudience  = "https://core.qovery.com"
	oAuthParamValueResponseType = "id_token token"
	oAuthUrlParamValueRedirect  = "http://localhost:" + strconv.Itoa(httpAuthPort) + "/authorization"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Do authentication",
	Long: `AUTH do auth on Qovery service. For example:

	qovery auth`,
	Run: func(cmd *cobra.Command, args []string) {
			api.QoveryAuthentication()
	},
}

func init() {
	RootCmd.AddCommand(authCmd)
}
