package cmd

import (
	"fmt"
	"github.com/qovery/qovery-cli/io"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information for the Qovery CLI",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("%s\n", io.GetCurrentVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
