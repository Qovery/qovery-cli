package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
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
	},
}

/*func init() {
	checkoutCmd.PersistentFlags().StringVarP(&ConfigurationDirectoryRoot, "configuration-directory-root", "c", ".", "Your configuration directory root path")

	RootCmd.AddCommand(checkoutCmd)
}*/
