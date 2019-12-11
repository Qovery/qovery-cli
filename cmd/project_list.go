package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"qovery.go/api"
)

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long: `LIST show all available projects. For example:

	qovery project list`,

	Run: func(cmd *cobra.Command, args []string) {
		output := []string{
			"name | region",
		}

		projects := api.ListProjects()

		if projects.Results == nil || len(projects.Results) == 0 {
			fmt.Println(columnize.SimpleFormat(output))
			return
		}

		for _, p := range projects.Results {
			output = append(output, p.Name+" | "+"eu-west-1")
		}

		fmt.Println(columnize.SimpleFormat(output))
	},
}

func init() {
	projectCmd.AddCommand(projectListCmd)
}
