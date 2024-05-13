package utils

import (
	"strings"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Capture(command *cobra.Command) {
	ph, err := posthog.NewWithConfig(
		"phc_IgdG1K2GveDUte1gJ6hlwNbFHCv9nViWETUyLMU7ciq",
		posthog.Config{
			Endpoint: "https://app.posthog.com",
		},
	)
	if err != nil {
		return
	}
	defer ph.Close()

	ctx, err := GetOrSetCurrentContext()
	if err != nil {
		return
	}

	properties := ctx.ToPosthogProperties()
	properties["command"] = commandName(command)
	flags := []string{}
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			flags = append(flags, flag.Name)
		}
	})
	properties["flags"] = strings.Join(flags, " ")

	err = ph.Enqueue(posthog.Capture{
		DistinctId: string(ctx.User),
		Event:      "cli-command-execution",
		Timestamp:  time.Now(),
		Properties: properties,
	})
	if err != nil {
		return
	}
}

func commandName(command *cobra.Command) string {
	if command.HasParent() {
		return commandName(command.Parent()) + " " + command.Name()
	} else {
		return command.Name()
	}
}
