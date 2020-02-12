package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"qovery.go/api"
	"qovery.go/util"
)

var checkoutCmd = &cobra.Command{
	Use:   "checkout",
	Short: "Equivalent to 'git checkout' but with Qovery magic sauce",
	Long: `CHECKOUT performs 'git checkout' action and set Qovery properties to target the right environment . For example:

	qovery checkout`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 1 {
			fmt.Println("qovery checkout <branch>")
			os.Exit(1)
		}

		branch := args[0]
		// checkout branch
		util.Checkout(branch)

		LoadAndSaveLocalConfiguration(ConfigurationDirectoryRoot)
	},
}

func getApplicationConfigByName(projectId string, branchName string, appName string) map[string]interface{} {
	return filterApplicationsByName(api.ListApplicationsRaw(projectId, branchName), appName)
}

func filterApplicationsByName(applications map[string]interface{}, appName string) map[string]interface{} {
	if val, ok := applications["results"]; ok {
		results := val.([]interface{})
		for _, application := range results {
			a := application.(map[string]interface{})
			if name, found := a["name"]; found && name == appName {
				return a
			}
		}
	}
	return nil
}

/*func init() {
	checkoutCmd.PersistentFlags().StringVarP(&ConfigurationDirectoryRoot, "configuration-directory-root", "c", ".", "Your configuration directory root path")

	RootCmd.AddCommand(checkoutCmd)
}*/

func LoadAndSaveLocalConfiguration(configurationDirectoryRoot string) {
	api.DeleteLocalConfiguration(configurationDirectoryRoot)

	qConf := util.CurrentQoveryYML()
	branchName := util.CurrentBranchName()
	projectName := qConf.Application.Project
	appName := qConf.Application.Name

	if branchName == "" || projectName == "" {
		fmt.Println("The current directory is not a Qovery project. Please consider using 'qovery init'")
		os.Exit(1)
	}

	project := api.GetProjectByName(projectName)
	if project.Id == "" {
		fmt.Println("The project does not exist. Are you well authenticated with the right user? Do 'qovery auth' to be sure")
		os.Exit(1)
	}

	if configMap := filterApplicationsByName(api.ListApplicationsRaw(project.Id, branchName), appName); configMap != nil {
		api.SaveLocalConfiguration(configurationDirectoryRoot, configMap)
	} else {
		log.Printf("application '%s' not found", appName)
	}
}
