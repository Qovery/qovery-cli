package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/io"
	"strings"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `LIST show all available environments. For example:

	qovery environment list`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			qoveryYML, err := io.CurrentQoveryYML()
			if err != nil {
				io.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}
		environments := io.ListEnvironments(io.GetProjectByName(ProjectName).Id)

		table := io.GetTable()
		table.SetHeader([]string{"branch", "status", "endpoints", "region", "applications", "databases"})

		if environments.Results == nil || len(environments.Results) == 0 {
			table.Append([]string{"", "", "", "", "", ""})
		} else {
			for _, a := range environments.Results {
				databaseName := "none"
				if a.Databases != nil {
					databaseName = strings.Join(a.GetDatabaseNames(), ", ")
				}

				applicationName := "none"
				if a.Applications != nil {
					applicationName = strings.Join(a.GetApplicationNames(), ", ")
				}

				//output = append(output,
				table.Append([]string{
					a.Name,
					a.Status.GetColoredCodeMessage(),
					strings.Join(a.GetConnectionURIs(), ", "),
					fmt.Sprintf("%s (%s)", a.CloudProviderRegion.FullName, a.CloudProviderRegion.Description),
					applicationName,
					databaseName,
				})
			}
		}
		table.Render()
	},
}

func init() {
	environmentListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	environmentCmd.AddCommand(environmentListCmd)
}
