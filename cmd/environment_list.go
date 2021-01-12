package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"qovery-cli/io"
	"strings"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `LIST show all available environments. For example:

	qovery environment list`,

	Run: func(cmd *cobra.Command, args []string) {
		LoadCommandOptions(cmd, true, true, false, false, true)
		environments := io.ListEnvironments(io.GetProjectByName(ProjectName, OrganizationName).Id)

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
					a.Status.GetColoredStatus(),
					strings.Join(a.GetConnectionURIs(), ", "),
					fmt.Sprintf("%s (%s)", a.Kubernetes.CloudProviderRegion.FullName, a.Kubernetes.CloudProviderRegion.Description),
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
	environmentListCmd.PersistentFlags().StringVarP(&OrganizationName, "organization", "o", "", "Your organization name")
	environmentCmd.AddCommand(environmentListCmd)
}
