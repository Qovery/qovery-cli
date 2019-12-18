package cmd

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"log"
	"os"
	"qovery.go/util"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to your remote environment (databases, brokers...)",
	Long: `CONNECT open a secured tunnel between your local machine and your remote environment. Perfect to connect to your remote databases and other services example:

	qovery connect`,
	Run: func(cmd *cobra.Command, args []string) {
		if !hasFlagChanged(cmd) {
			BranchName = util.CurrentBranchName()
			ProjectName = util.CurrentQoveryYML().Application.Project

			if BranchName == "" || ProjectName == "" {
				fmt.Println("The current directory is not a Qovery project (-h for help)")
				os.Exit(0)
			}
		}

		ReloadEnvironment()
		watchForBranchCheckout()
	},
}

func init() {
	connectCmd.PersistentFlags().StringVarP(&ProjectName, "project", "p", "", "Your project name")
	connectCmd.PersistentFlags().StringVarP(&BranchName, "environment", "e", "", "Your environment name")

	RootCmd.AddCommand(connectCmd)
}

func watchForBranchCheckout() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op&fsnotify.Create == fsnotify.Create {
					ReloadEnvironment()
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

func ReloadEnvironment() {
	branchName := util.CurrentBranchName()
	log.Printf("reload %s environment: IN PROGRESS\n", branchName)
	LoadAndSaveLocalConfiguration()
	log.Printf("reload %s environment: DONE\n", branchName)
}
