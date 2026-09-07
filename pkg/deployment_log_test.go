package pkg

import (
	"strings"
	"testing"
	"time"

	"github.com/qovery/qovery-client-go"
)

func strPtr(s string) *string { return &s }

// deploymentLogOpts describes a synthetic log line for the tests below.
type deploymentLogOpts struct {
	step            string
	stageName       string
	transmitterId   string
	transmitterType string
	transmitterName string
	message         string
	errUserMessage  string
	errHint         string
	errLink         string
	errTag          string
}

func newDeploymentLog(o deploymentLogOpts) qovery.EnvironmentLogs {
	l := qovery.EnvironmentLogs{
		Type:      "debug",
		Timestamp: time.Date(2026, 9, 6, 14, 3, 12, 418000000, time.UTC),
		Details: qovery.EnvironmentLogsDetails{
			Stage: &qovery.EnvironmentLogsDetailsStage{
				Step: strPtr(o.step),
				Name: *qovery.NewNullableString(strPtr(o.stageName)),
			},
			Transmitter: &qovery.EnvironmentLogsDetailsTransmitter{
				Id:   strPtr(o.transmitterId),
				Type: strPtr(o.transmitterType),
				Name: strPtr(o.transmitterName),
			},
		},
		Message: *qovery.NewNullableEnvironmentLogsMessage(&qovery.EnvironmentLogsMessage{
			SafeMessage: strPtr(o.message),
		}),
	}

	if o.errUserMessage != "" || o.errHint != "" || o.errLink != "" || o.errTag != "" {
		l.Error = *qovery.NewNullableEnvironmentLogsError(&qovery.EnvironmentLogsError{
			Tag:            strPtr(o.errTag),
			UserLogMessage: strPtr(o.errUserMessage),
			HintMessage:    strPtr(o.errHint),
			Link:           strPtr(o.errLink),
		})
	}

	return l
}

const (
	serviceUnderTest = "11111111-1111-1111-1111-111111111111"
	otherService     = "22222222-2222-2222-2222-222222222222"
)

func TestFilterDeploymentLogs(t *testing.T) {
	preCheck := newDeploymentLog(deploymentLogOpts{
		step: PreCheckStep, transmitterType: "Environment", message: "pre-check",
	})
	envWide := newDeploymentLog(deploymentLogOpts{
		step: "Deployed", transmitterType: "Environment", message: "env level",
	})
	taskManager := newDeploymentLog(deploymentLogOpts{
		step: "Queued", transmitterType: "TaskManager", message: "queued",
	})
	mine := newDeploymentLog(deploymentLogOpts{
		step: "Build", transmitterType: "Application", transmitterId: serviceUnderTest,
		transmitterName: "my-app", message: "mine",
	})
	theirs := newDeploymentLog(deploymentLogOpts{
		step: "Build", transmitterType: "Application", transmitterId: otherService,
		transmitterName: "their-app", message: "theirs",
	})
	failed := newDeploymentLog(deploymentLogOpts{
		step: "Deployed", transmitterType: "Application", transmitterId: serviceUnderTest,
		transmitterName: "my-app", message: "boom", errUserMessage: "liveness probe failed",
	})

	all := []qovery.EnvironmentLogs{preCheck, envWide, taskManager, mine, theirs, failed}

	tests := []struct {
		name   string
		filter DeploymentLogFilter
		want   []string
	}{
		{
			name:   "zero value keeps everything",
			filter: DeploymentLogFilter{},
			want:   []string{"pre-check", "env level", "queued", "mine", "theirs", "boom"},
		},
		{
			name:   "service scope keeps env-wide transmitters and drops pre-check and other services",
			filter: DeploymentLogFilter{ServiceID: serviceUnderTest},
			want:   []string{"env level", "queued", "mine", "boom"},
		},
		{
			name:   "pre-check only is the complement of a service scope",
			filter: DeploymentLogFilter{PreCheckOnly: true},
			want:   []string{"pre-check"},
		},
		{
			name:   "errors only",
			filter: DeploymentLogFilter{ErrorsOnly: true},
			want:   []string{"boom"},
		},
		{
			name:   "errors only combined with a service scope",
			filter: DeploymentLogFilter{ServiceID: serviceUnderTest, ErrorsOnly: true},
			want:   []string{"boom"},
		},
		{
			name:   "unknown service keeps only the env-wide lines",
			filter: DeploymentLogFilter{ServiceID: "33333333-3333-3333-3333-333333333333"},
			want:   []string{"env level", "queued"},
		},
		{
			name:   "service scope by name behaves like scoping by id",
			filter: DeploymentLogFilter{ServiceName: "my-app"},
			want:   []string{"env level", "queued", "mine", "boom"},
		},
		{
			name:   "unknown service name keeps only the env-wide lines",
			filter: DeploymentLogFilter{ServiceName: "does-not-exist"},
			want:   []string{"env level", "queued"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterDeploymentLogs(all, tt.filter)

			if len(got) != len(tt.want) {
				t.Fatalf("got %d lines %v, want %d lines %v", len(got), messagesOf(got), len(tt.want), tt.want)
			}
			for i, want := range tt.want {
				if msg := got[i].Message.Get().GetSafeMessage(); msg != want {
					t.Errorf("line %d: got %q, want %q", i, msg, want)
				}
			}
		})
	}
}

func messagesOf(logs []qovery.EnvironmentLogs) []string {
	msgs := make([]string, 0, len(logs))
	for _, l := range logs {
		msgs = append(msgs, l.Message.Get().GetSafeMessage())
	}
	return msgs
}

func TestFilterDeploymentLogsHandlesMissingDetails(t *testing.T) {
	// The API marks details.stage and details.transmitter as optional, so a line may
	// carry neither. Filtering must not panic on those.
	bare := qovery.EnvironmentLogs{Type: "debug", Timestamp: time.Now()}

	if got := FilterDeploymentLogs([]qovery.EnvironmentLogs{bare}, DeploymentLogFilter{}); len(got) != 1 {
		t.Errorf("unfiltered: got %d lines, want 1", len(got))
	}
	if got := FilterDeploymentLogs([]qovery.EnvironmentLogs{bare}, DeploymentLogFilter{ServiceID: serviceUnderTest}); len(got) != 0 {
		t.Errorf("service scoped: got %d lines, want 0", len(got))
	}
	if got := FormatDeploymentLogLine(bare); !strings.Contains(got, "| - | - |") {
		t.Errorf("formatting a bare line: got %q, want dashes for the missing fields", got)
	}
}

func TestFormatDeploymentLogLine(t *testing.T) {
	plain := newDeploymentLog(deploymentLogOpts{
		step: "Build", transmitterType: "Application", transmitterName: "my-app", message: "building image",
	})

	want := "| 2026-09-06 14:03:12.418 | Build | Application/my-app | building image"
	if got := FormatDeploymentLogLine(plain); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestFormatDeploymentLogLineWithError(t *testing.T) {
	failed := newDeploymentLog(deploymentLogOpts{
		step: "Deployed", transmitterType: "Application", transmitterName: "my-app",
		message:        "deployment failed",
		errUserMessage: "liveness probe failed",
		errHint:        "check the probe path",
		errLink:        "https://hub.qovery.com/docs/probes",
	})

	got := FormatDeploymentLogLine(failed)
	lines := strings.Split(got, "\n")

	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4:\n%s", len(lines), got)
	}
	for i, want := range []string{
		"| 2026-09-06 14:03:12.418 | Deployed | Application/my-app | deployment failed",
		"    error: liveness probe failed",
		"    hint: check the probe path",
		"    link: https://hub.qovery.com/docs/probes",
	} {
		if lines[i] != want {
			t.Errorf("line %d: got %q, want %q", i, lines[i], want)
		}
	}
}

func TestFormatDeploymentLogLineOmitsEmptyErrorFields(t *testing.T) {
	// An error with no hint and no link must not print empty follow-up lines.
	failed := newDeploymentLog(deploymentLogOpts{
		step: "Build", transmitterType: "Application", transmitterName: "my-app",
		message: "build failed", errUserMessage: "Dockerfile not found",
	})

	got := FormatDeploymentLogLine(failed)
	if lines := strings.Split(got, "\n"); len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), got)
	}
	if !strings.HasSuffix(got, "    error: Dockerfile not found") {
		t.Errorf("got %q, want it to end with the error follow-up line", got)
	}
}

func TestDeploymentLogJSON(t *testing.T) {
	l := newDeploymentLog(deploymentLogOpts{
		step: "Deployed", stageName: "backend", transmitterType: "Application",
		transmitterName: "my-app", transmitterId: serviceUnderTest,
		message: "deployment failed", errTag: "K8S_CANNOT_APPLY_CONFIG",
		errUserMessage: "liveness probe failed", errHint: "check the probe path",
		errLink: "https://hub.qovery.com/docs/probes",
	})

	got := DeploymentLogJSON(l)

	for field, want := range map[string]string{
		"stage":            "backend",
		"step":             "Deployed",
		"transmitter":      "my-app",
		"transmitter_type": "Application",
		"message":          "deployment failed",
	} {
		if got[field] != want {
			t.Errorf("%s: got %v, want %q", field, got[field], want)
		}
	}

	ts, ok := got["timestamp"].(*string)
	if !ok || ts == nil {
		t.Fatalf("timestamp: got %v, want an ISO-8601 string", got["timestamp"])
	}
	if *ts != "2026-09-06T14:03:12.418Z" {
		t.Errorf("timestamp: got %q, want %q", *ts, "2026-09-06T14:03:12.418Z")
	}

	errField, ok := got["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("error: got %v, want a nested object", got["error"])
	}
	for field, want := range map[string]string{
		"tag":              "K8S_CANNOT_APPLY_CONFIG",
		"user_log_message": "liveness probe failed",
		"hint_message":     "check the probe path",
		"link":             "https://hub.qovery.com/docs/probes",
	} {
		if errField[field] != want {
			t.Errorf("error.%s: got %v, want %q", field, errField[field], want)
		}
	}
}

func TestDeploymentLogJSONOmitsErrorWhenAbsent(t *testing.T) {
	l := newDeploymentLog(deploymentLogOpts{step: "Build", message: "building image"})

	if got := DeploymentLogJSON(l); got["error"] != nil {
		t.Errorf("error: got %v, want the key to be absent", got["error"])
	}
}

func TestDeploymentLogFilterIsServiceScoped(t *testing.T) {
	for _, tt := range []struct {
		name   string
		filter DeploymentLogFilter
		want   bool
	}{
		{"zero value", DeploymentLogFilter{}, false},
		{"by id", DeploymentLogFilter{ServiceID: serviceUnderTest}, true},
		{"by name", DeploymentLogFilter{ServiceName: "my-app"}, true},
		{"errors only is not a service scope", DeploymentLogFilter{ErrorsOnly: true}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.IsServiceScoped(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeploymentLogServices(t *testing.T) {
	logs := []qovery.EnvironmentLogs{
		newDeploymentLog(deploymentLogOpts{step: PreCheckStep, transmitterType: "Environment", transmitterName: "Development"}),
		newDeploymentLog(deploymentLogOpts{step: "Deployed", transmitterType: "Environment", transmitterName: "Development"}),
		newDeploymentLog(deploymentLogOpts{step: "Queued", transmitterType: "TaskManager", transmitterName: "tm"}),
		newDeploymentLog(deploymentLogOpts{step: "Build", transmitterType: "Terraform", transmitterName: "S3"}),
		newDeploymentLog(deploymentLogOpts{step: "Build", transmitterType: "Application", transmitterName: "my-app"}),
		newDeploymentLog(deploymentLogOpts{step: "Deployed", transmitterType: "Application", transmitterName: "my-app"}),
	}

	got := DeploymentLogServices(logs)
	want := []string{"Application/my-app", "Terraform/S3"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestHasServiceSpecificLines(t *testing.T) {
	envOnly := []qovery.EnvironmentLogs{
		newDeploymentLog(deploymentLogOpts{step: "Deployed", transmitterType: "Environment"}),
		newDeploymentLog(deploymentLogOpts{step: "Queued", transmitterType: "TaskManager"}),
	}
	if HasServiceSpecificLines(envOnly) {
		t.Error("environment-only lines: got true, want false")
	}

	withService := append(envOnly, newDeploymentLog(deploymentLogOpts{
		step: "Build", transmitterType: "Application", transmitterName: "my-app",
	}))
	if !HasServiceSpecificLines(withService) {
		t.Error("with a service line: got false, want true")
	}
}
