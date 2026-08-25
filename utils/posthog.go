package utils

import (
	"os"
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
const PhaseFinishedEventName = "cli-command-phase-finished"

type CommandExecutionProperties struct {
	AttemptID        string
	WorkflowType     string
	Implementation   string
	ClusterID        string
	Phase            string
	Result           string
	DurationMillis   int64
	ErrorCode        string
	SafeErrorMessage string
}

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

func CaptureCommandExecution(command *cobra.Command, event string, execution CommandExecutionProperties) {
	CaptureWithEventAndProperties(command, event, commandExecutionPostHogProperties(execution))
}

func commandExecutionPostHogProperties(execution CommandExecutionProperties) posthog.Properties {
	properties := posthog.Properties{
		"attempt_id":     execution.AttemptID,
		"workflow_type":  execution.WorkflowType,
		"implementation": execution.Implementation,
	}
	if execution.AttemptID != "" {
		properties["$session_id"] = execution.AttemptID
	}
	if execution.Result != "" {
		properties["result"] = execution.Result
	}
	if execution.ClusterID != "" {
		properties["cluster_id"] = execution.ClusterID
	}
	if execution.Phase != "" {
		properties["phase"] = execution.Phase
	}
	if execution.DurationMillis > 0 {
		properties["duration_ms"] = execution.DurationMillis
	}
	if execution.ErrorCode != "" {
		properties["error_code"] = execution.ErrorCode
	}
	if execution.SafeErrorMessage != "" {
		properties["safe_error_message"] = execution.SafeErrorMessage
	}

	return properties
}

func CaptureWithEventAndProperties(command *cobra.Command, event string, properties posthog.Properties) {
	// Do not track the command execution in Qovery telemetry.
	if flag := os.Getenv("QOVERY_TELEMETRY"); strings.EqualFold(flag, "false") {
		return
	}

	ph, err := posthog.NewWithConfig(
		"phc_IgdG1K2GveDUte1gJ6hlwNbFHCv9nViWETUyLMU7ciq",
		posthog.Config{
			Endpoint: "https://e.qovery.com",
		},
	)

	if err != nil {
		return
	}

	defer func() {
		_ = ph.Close()
	}()

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
		Set("cli_version", Version).
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
