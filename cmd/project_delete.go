package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var projectDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete project",
	Long: `DELETE delete project. For example:

	qovery project delete <name>`,

	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}

		fmt.Println("OK")
	},
}

func init() {
	projectCmd.AddCommand(projectDeleteCmd)
}
