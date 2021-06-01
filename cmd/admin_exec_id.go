package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"qovery-cli/io"
)

var execIdCmd = &cobra.Command{
	Use: "exec-id",
	Short: "Get all environment info witn an exectution id",
	Run: func(cmd *cobra.Command, args []string) {
		getInfos(args)
	},
}

func init(){
	adminCmd.AddCommand(execIdCmd)
}

func getInfos(args []string) {
	if len(args) < 1 || args[0] == "" {
		log.Error("You enter a wrong execution ID.")
		return
	}

	log.Info(io.GetInfos(args[0]))
}
