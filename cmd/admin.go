package cmd

import (
	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{Use: "admin", Hidden: true}

func init() {
	rootCmd.AddCommand(adminCmd)
}
