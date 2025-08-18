package cmd

import (
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg"
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
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.Flags().BoolVarP(&headless, "headless", "", false, "Headless auth")
	authCmd.Flags().BoolVarP(&skipVersionCheck, "skipVersionCheck", "", false, "Skip CLI version check during authentication")
}
