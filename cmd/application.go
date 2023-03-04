package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var applicationName string
var applicationCommitId string
var applicationBranch string
var targetApplicationName string

var applicationCmd = &cobra.Command{
	Use:   "application",
	Short: "Manage applications",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(applicationCmd)
}
