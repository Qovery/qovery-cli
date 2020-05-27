package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var headless bool

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Do authentication",
	Long: `AUTH do auth on Qovery service. For example:

	qovery auth`,
	Run: func(cmd *cobra.Command, args []string) {
		io.DoRequestUserToAuthenticate(headless)
	},
}

func init() {
	authCmd.Flags().BoolVar(&headless, "headless", false, "Headless authentication")
	RootCmd.AddCommand(authCmd)
}
