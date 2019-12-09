package cmd

import (
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Perform project actions",
	Long: `PROJECT performs actions on project. For example:

	qovery project`,
}

func init() {
	RootCmd.AddCommand(projectCmd)
}
