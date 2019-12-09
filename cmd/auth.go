package cmd

import (
	"fmt"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"qovery.go/util"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Do authentication",
	Long: `AUTH do auth on Qovery service. For example:

	qovery auth`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO link to web auth
		_ = browser.OpenURL("https://auth.qovery.com/login?state=g6Fo2SBVenl5X1dwYjFsV0dtaE03aHJmMTVfdl95RnhUNVVDbKN0aWTZIDFQWVhLRWNkS2xfUUx" +
			"pUWY3enFVN1BYY3paX2pDNHJOo2NpZNkgTUoyU0pwdTEyUHhJemdtYzV6NVk3TjhtNU1uYUY3WTA&client=MJ2SJpu12PxIzgmc5z5Y7N8m5MnaF7Y0" +
			"&protocol=oauth2&response_type=id_token%20token&audience=https://core.qovery.com&redirect_uri=https%3A%2F%2Fcloud.qovery.com")

		authorizationCode := util.AskForInput(false, "Authorization code")
		fmt.Println(authorizationCode)
	},
}

func init() {
	RootCmd.AddCommand(authCmd)
}
