package cmd

import (
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Do project initialization to use Qovery",
	Long: `INIT do project initialization to use Qovery within the current directory. For example:
	
	qovery init`,
	Run: func(cmd *cobra.Command, args []string) {
		runInit()
	},
}

var templateFlag string

func init() {
	RootCmd.AddCommand(initCmd)
}

func runInit() {
	io.PrintSolution("Qovery User Interface is ready! Opening your browser to initialize a project.")
	_ = browser.OpenURL("https://console.qovery.com/platform/organization/p54ch1udm62fh71t/projects/new-project/name")
}
