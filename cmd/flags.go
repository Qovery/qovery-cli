package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//flags used by more than 1 command
var DebugFlag bool
var Name string
var ProjectName string
var EnvironmentName string

func hasFlagChanged(cmd *cobra.Command) bool {
	flagChanged := false

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			flagChanged = true
		}
	})

	return flagChanged
}
