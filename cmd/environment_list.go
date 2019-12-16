package cmd

import (
	"fmt"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strconv"
	"strings"
)

var environmentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Long: `LIST show all available environments. For example:

	qovery environment list`,

	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			ProjectName = util.CurrentQoveryYML().Application.Project

			if ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		output := []string{
			"branch | status | endpoints | applications | databases | brokers | storage",
		}

		// TODO check nil
		aggEnvs := api.ListBranches(api.GetProjectByName(ProjectName).Id)

		if aggEnvs.Results == nil || len(aggEnvs.Results) == 0 {
			fmt.Println(columnize.SimpleFormat(output))
			return
		}

		for _, a := range aggEnvs.Results {
			output = append(output, a.BranchId+" | "+a.Status+" | "+strings.Join(a.ConnectionURIs, ", ")+" | "+strconv.Itoa(*a.TotalApplications)+
				" | "+strconv.Itoa(*a.TotalDatabases)+" | "+strconv.Itoa(*a.TotalBrokers)+" | "+strconv.Itoa(*a.TotalStorage))
		}

		fmt.Println(columnize.SimpleFormat(output))
	},
}

func init() {
	environmentListCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")

	environmentCmd.AddCommand(environmentListCmd)
}
