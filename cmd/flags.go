package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//flags used by more than 1 command
var (
	DebugFlag                  bool
	WatchFlag                  bool
	FollowFlag                 bool
	Name                       string
	ApplicationName            string
	ProjectName                string
	BranchName                 string
	Tail                       int
	ConfigurationDirectoryRoot string
)

func hasFlagChanged(cmd *cobra.Command) bool {
	flagChanged := false

	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed && flag.Name != "watch" && flag.Name != "follow" && flag.Name != "tail" {
			flagChanged = true
		}
	})

	return flagChanged
}
