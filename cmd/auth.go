package cmd

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/pkg/skills"
	"github.com/qovery/qovery-cli/utils"
)

var headless bool
var skipVersionCheck bool

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Log in to Qovery",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		previousToken := currentAccessToken()
		pkg.DoRequestUserToAuthenticate(headless, skipVersionCheck)
		// Right after signing in is when the user is most likely to set up their AI agent
		if hasNewSession(previousToken) {
			skills.OfferAfterAuth(cmd, headless)
		}
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.Flags().BoolVarP(&headless, "headless", "", false, "Headless auth")
	authCmd.Flags().BoolVarP(&skipVersionCheck, "skipVersionCheck", "", false, "Skip CLI version check during authentication")
}

func currentAccessToken() utils.AccessToken {
	ctx, err := utils.GetCurrentContext()
	if err != nil {
		return ""
	}
	return ctx.AccessToken
}

// hasNewSession reports whether this run stored a new valid session. The browser flow can return without signing in
// (e.g. when it cannot listen locally), leaving a previous session in place
func hasNewSession(previousToken utils.AccessToken) bool {
	ctx, err := utils.GetCurrentContext()
	return err == nil && ctx.AccessToken != "" && ctx.AccessToken != previousToken && ctx.AccessTokenExpiration.After(time.Now())
}
