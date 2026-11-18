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
		pkg.DoRequestUserToAuthenticate(headless, skipVersionCheck)
		// Right after signing in is when the user is most likely to set up their AI agent
		if isSignedIn() {
			skills.OfferAfterAuth(cmd)
		}
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.Flags().BoolVarP(&headless, "headless", "", false, "Headless auth")
	authCmd.Flags().BoolVarP(&skipVersionCheck, "skipVersionCheck", "", false, "Skip CLI version check during authentication")
}

// isSignedIn reports whether a valid session is stored. The browser flow returns without one when it cannot listen locally
func isSignedIn() bool {
	ctx, err := utils.GetCurrentContext()
	return err == nil && ctx.AccessToken != "" && ctx.AccessTokenExpiration.After(time.Now())
}
