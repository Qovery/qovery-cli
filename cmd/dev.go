package cmd

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"log"
	"os"
	"qovery.go/util"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Dev to your remote environment (databases, brokers...)",
	Long: `DEV open a secured tunnel between your local machine and your remote environment. Perfect to dev with your remote databases and other services example:

	qovery dev`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			qoveryYML, err := util.CurrentQoveryYML()
			if err != nil {
				util.PrintError("No qovery configuration file found")
				os.Exit(1)
			}
			ProjectName = qoveryYML.Application.Project
		}

		ReloadEnvironment(ConfigurationDirectoryRoot)
		watchForBranchCheckout()
	},
}

/*func init() {
	devCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	devCmd.PersistentFlags().StringVarP(&BranchName, "environment", "e", "", "Your environment name")
	devCmd.PersistentFlags().StringVarP(&ConfigurationDirectoryRoot, "configuration-directory-root", "c", ".", "Your configuration directory root path")

	RootCmd.AddCommand(devCmd)
}*/

func watchForBranchCheckout() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer watcher.Close()

	done := make(chan bool)
	defer close(done)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Create == fsnotify.Create {
					ReloadEnvironment(ConfigurationDirectoryRoot)
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(".git/HEAD")
	if err != nil {
		fmt.Println(err)
	}

	<-done
}

func ReloadEnvironment(configurationDirectoryRoot string) {
	branchName := util.CurrentBranchName()
	log.Printf("reload %s environment: DONE\n", branchName)
}
