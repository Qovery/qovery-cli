package cmd

import (
	"github.com/spf13/cobra"
	"qovery.go/io"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Do authentication",
	Long: `AUTH do auth on Qovery service. For example:

	qovery auth`,
	Run: func(cmd *cobra.Command, args []string) {
		io.DoRequestUserToAuthenticate()
	},
}

func init() {
	RootCmd.AddCommand(authCmd)
}
