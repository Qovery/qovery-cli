package utils

import (
	"runtime"
	"strings"
	"time"

	"github.com/posthog/posthog-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const DefaultEventName = "cli-command-execution"
const EndOfExecutionEventName = "cli-command-execution-end"
const EndOfExecutionErrorEventName = "cli-command-execution-error"

func Capture(command *cobra.Command) {
	CaptureWithEvent(command, DefaultEventName)
}

func CaptureError(command *cobra.Command, stout string, stderr string) {
	properties := posthog.Properties{
		"stdout": stout,
		"stderr": stderr,
	}

	CaptureWithEventAndProperties(command, EndOfExecutionErrorEventName, properties)
}

func CaptureWithEvent(command *cobra.Command, event string) {
	CaptureWithEventAndProperties(command, event, posthog.Properties{})
}

func CaptureWithEventAndProperties(command *cobra.Command, event string, properties posthog.Properties) {
	ph, err := posthog.NewWithConfig(
		"phc_IgdG1K2GveDUte1gJ6hlwNbFHCv9nViWETUyLMU7ciq",
		posthog.Config{
			Endpoint: "https://phprox.qovery.com",
		},
	)

	if err != nil {
		return
	}

	defer ph.Close()

	ctx, err := GetCurrentContext()
	if err != nil {
		return
	}

	tokenType := "jwt"
	if strings.HasPrefix(string(ctx.AccessToken), "qov_") {
		tokenType = "static"
	}

	mProperties := properties.
		Set("organization", ctx.OrganizationName).
		Set("organization_id", ctx.OrganizationId).
		Set("project", ctx.ProjectName).
		Set("project_id", ctx.ProjectId).
		Set("environment", ctx.EnvironmentName).
		Set("environment_id", ctx.EnvironmentId).
		Set("service", ctx.ServiceName).
		Set("service_id", ctx.ServiceId).
		Set("token_type", tokenType).
		Set("os", runtime.GOOS).
		Set("arch", runtime.GOARCH).
		Set("command", commandName(command))

	flags := []string{}
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			flags = append(flags, flag.Name)
		}
	})
	properties["flags"] = strings.Join(flags, " ")

	err = ph.Enqueue(posthog.Capture{
		DistinctId: string(ctx.User),
		Event:      event,
		Timestamp:  time.Now(),
		Properties: mProperties,
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
