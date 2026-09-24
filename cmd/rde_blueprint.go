package cmd

import (
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var rdeBlueprintCmd = &cobra.Command{
	Use:   "blueprint",
	Short: "Manage RDE blueprints",
	Long: `Manage RDE blueprint projects and environments.

A blueprint is a project with a template environment that serves as the source
for cloning new Remote Development Environments. Blueprints are identified by
a project-level environment variable BLUEPRINT_PROJECT_ID.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rdeCmd.AddCommand(rdeBlueprintCmd)
}
