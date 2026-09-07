package pkg

import (
	"fmt"
	"sort"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

// PreCheckStep is the value of `details.stage.step` carried by the pre-check log lines.
// Pre-check runs before any stage, so those lines are excluded from a service-scoped
// view and are the exact complement of it.
const PreCheckStep = "PreCheck"

// environmentWideTransmitters are the transmitter types that emit log lines concerning the
// deployment as a whole rather than a single service. They are kept even when the view is
// scoped to one service, because they carry the surrounding context (stage boundaries,
// queueing, cancellation).
var environmentWideTransmitters = map[string]bool{
	"Environment": true,
	"TaskManager": true,
}

// DeploymentLogFilter narrows a deployment log payload down to what the caller asked for.
// The zero value keeps every line.
type DeploymentLogFilter struct {
	// ServiceID scopes the output to the service with that id.
	ServiceID string
	// ServiceName scopes the output to the service with that name, matched against the
	// transmitter of each line. Service names are unique within an environment, and matching
	// on the payload rather than resolving the name through the API keeps every service type
	// working -- including Terraform services, which the shared CLI resolver cannot look up.
	ServiceName string
	// PreCheckOnly keeps only the pre-check lines. Mutually exclusive with the service scope.
	PreCheckOnly bool
	// ErrorsOnly keeps only the lines carrying an error.
	ErrorsOnly bool
}

// IsServiceScoped reports whether f narrows the output to a single service.
func (f DeploymentLogFilter) IsServiceScoped() bool {
	return f.ServiceID != "" || f.ServiceName != ""
}

// FilterDeploymentLogs applies f to logs, preserving order.
//
// The service predicate mirrors the one the Qovery console applies client-side, with one
// documented difference: the console also requires the line's `details.stage.id` to match
// the deployment stage the service belongs to, which neither the CLI nor the MCP server has
// in hand without an extra lookup. As a result a few stage-level lines belonging to other
// stages may show up in a service-scoped view.
func FilterDeploymentLogs(logs []qovery.EnvironmentLogs, f DeploymentLogFilter) []qovery.EnvironmentLogs {
	filtered := make([]qovery.EnvironmentLogs, 0, len(logs))

	for _, l := range logs {
		if !keepDeploymentLog(l, f) {
			continue
		}
		filtered = append(filtered, l)
	}

	return filtered
}

func keepDeploymentLog(l qovery.EnvironmentLogs, f DeploymentLogFilter) bool {
	isPreCheck := l.Details.Stage.GetStep() == PreCheckStep

	if f.PreCheckOnly && !isPreCheck {
		return false
	}

	if f.IsServiceScoped() {
		if isPreCheck {
			return false
		}
		if !isEnvironmentWide(l) && !matchesService(l, f) {
			return false
		}
	}

	if f.ErrorsOnly && !hasDeploymentLogError(l) {
		return false
	}

	return true
}

func hasDeploymentLogError(l qovery.EnvironmentLogs) bool {
	return l.Error.IsSet() && l.Error.Get() != nil
}

// isEnvironmentWide reports whether l concerns the deployment as a whole rather than one
// service. Those lines are kept even in a service-scoped view, for the surrounding context.
func isEnvironmentWide(l qovery.EnvironmentLogs) bool {
	return environmentWideTransmitters[l.Details.Transmitter.GetType()]
}

func matchesService(l qovery.EnvironmentLogs, f DeploymentLogFilter) bool {
	transmitter := l.Details.Transmitter

	if f.ServiceID != "" && transmitter.GetId() == f.ServiceID {
		return true
	}
	if f.ServiceName != "" && transmitter.GetName() == f.ServiceName {
		return true
	}

	return false
}

// DeploymentLogServices lists the distinct services that emitted at least one of the given
// lines, as `Type/Name`, sorted. It backs the hint shown when a --service filter matched
// nothing, which is more useful than a bare "not found": it names what could be asked for.
func DeploymentLogServices(logs []qovery.EnvironmentLogs) []string {
	seen := make(map[string]bool)

	for _, l := range logs {
		if isEnvironmentWide(l) || l.Details.Stage.GetStep() == PreCheckStep {
			continue
		}
		if name := deploymentLogTransmitter(l); name != "" {
			seen[name] = true
		}
	}

	services := make([]string, 0, len(seen))
	for name := range seen {
		services = append(services, name)
	}
	sort.Strings(services)

	return services
}

// HasServiceSpecificLines reports whether logs contain at least one line emitted by a
// service rather than by the environment itself.
func HasServiceSpecificLines(logs []qovery.EnvironmentLogs) bool {
	for _, l := range logs {
		if !isEnvironmentWide(l) {
			return true
		}
	}
	return false
}

// FormatDeploymentLogLine renders one log line for a human reader, in the same
// pipe-delimited shape as the runtime logs printed by ExecLog:
//
//	| 2026-09-06 14:03:12.418 | Build | Application/my-app | building image
//
// A line carrying an error gets indented follow-up lines with the user-facing message,
// the hint and the documentation link -- the part someone debugging a failed deployment
// is actually after.
func FormatDeploymentLogLine(l qovery.EnvironmentLogs) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"| %s | %s | %s | %s",
		l.Timestamp.Format("2006-01-02 15:04:05.000"),
		orDash(l.Details.Stage.GetStep()),
		orDash(deploymentLogTransmitter(l)),
		l.Message.Get().GetSafeMessage(),
	))

	if hasDeploymentLogError(l) {
		e := l.Error.Get()
		for _, follow := range []struct {
			label string
			value string
		}{
			{"error", e.GetUserLogMessage()},
			{"hint", e.GetHintMessage()},
			{"link", e.GetLink()},
		} {
			if follow.value == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("\n    %s: %s", follow.label, follow.value))
		}
	}

	return sb.String()
}

// DeploymentLogJSON projects one log line onto the field names shared with the MCP
// `get_deployment_logs` tool, so both surfaces answer the same question the same way.
func DeploymentLogJSON(l qovery.EnvironmentLogs) map[string]interface{} {
	out := map[string]interface{}{
		"timestamp":        utils.ToIso8601(&l.Timestamp),
		"stage":            l.Details.Stage.GetName(),
		"step":             l.Details.Stage.GetStep(),
		"transmitter":      l.Details.Transmitter.GetName(),
		"transmitter_type": l.Details.Transmitter.GetType(),
		"message":          l.Message.Get().GetSafeMessage(),
	}

	if hasDeploymentLogError(l) {
		e := l.Error.Get()
		out["error"] = map[string]interface{}{
			"tag":              e.GetTag(),
			"user_log_message": e.GetUserLogMessage(),
			"hint_message":     e.GetHintMessage(),
			"link":             e.GetLink(),
		}
	}

	return out
}

// deploymentLogTransmitter renders the emitter of a line as `Type/Name`, degrading to
// whichever half is present.
func deploymentLogTransmitter(l qovery.EnvironmentLogs) string {
	transmitterType := l.Details.Transmitter.GetType()
	name := l.Details.Transmitter.GetName()

	switch {
	case transmitterType != "" && name != "":
		return transmitterType + "/" + name
	case name != "":
		return name
	default:
		return transmitterType
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
