package cmd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "qovery",
	Short: "The qovery command line interface.",
	Long:  `The qovery command line interface lets you manage your Qovery environment.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	fmt.Println("Execute called")

	if err := RootCmd.Execute(); err != nil {
		log.Debug(err.Error())
		os.Exit(-1)
	}

}

func init() {
	cobra.OnInitialize(initConfig)
	log.Debug("init called")

	RootCmd.PersistentFlags().BoolVar(&DebugFlag, "debug", false, "Enable debugging when true.")
}

func initConfig() {
	if DebugFlag {
		log.SetLevel(log.DebugLevel)
		log.Debug("debug flag is set to true")
	}

	if os.Getenv("GENERATE_BASH_COMPLETION") != "" {
		generateBashCompletion()
	}
}

func generateBashCompletion() {
	log.Debugf("generating bash completion script")
	file, err2 := os.Create("/tmp/qovery-bash-completion.out")
	if err2 != nil {
		fmt.Println("Error: ", err2.Error())
	}
	defer file.Close()
	RootCmd.GenBashCompletion(file)
}
