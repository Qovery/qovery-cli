package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Capture the real SDK's requests locally, without using the user's credentials
// or contacting the telemetry service. These tests must not run in parallel.
func setupTelemetryTest(t *testing.T, token string) <-chan []byte {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	ctx := QoveryContext{
		AccessToken:      AccessToken(token),
		RefreshToken:     "refresh-token-secret",
		User:             "test-user",
		OrganizationName: "test-org",
		OrganizationId:   "test-org-id",
	}
	contextPath, err := QoveryContextPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(contextPath), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contextPath, data, ContextFilePermissions); err != nil {
		t.Fatal(err)
	}

	requests := make(chan []byte, 16)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/batch/" {
			t.Errorf("unexpected telemetry request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading telemetry request: %v", err)
		}
		requests <- body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	originalTransport := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
		transport.CloseIdleConnections()
	})
	return requests
}

func TestTelemetryPreservesErrorOutput(t *testing.T) {
	for _, tokenType := range []string{"jwt", "static"} {
		for _, name := range []string{"up", "destroy"} {
			t.Run(tokenType+"/"+name, func(t *testing.T) {
				token := "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.test-signature"
				if tokenType == "static" {
					token = "qov_test-static-token-secret"
				}
				requests := setupTelemetryTest(t, token)
				t.Setenv("QOVERY_TELEMETRY", "true")

				root := &cobra.Command{Use: "qovery", SilenceErrors: true, SilenceUsage: true}
				demo := &cobra.Command{Use: "demo"}
				command := &cobra.Command{
					Use: name + " [args]",
					RunE: func(cmd *cobra.Command, args []string) error {
						cmd.Println("demo command output")
						cmd.PrintErrln("demo script failed")
						return fmt.Errorf("exit status 1")
					},
				}
				root.AddCommand(demo)
				demo.AddCommand(command)
				command.Flags().String("token", "", "Authentication token")
				root.SetArgs([]string{"demo", name, "--token", token, "argument-secret"})
				var stdout, stderr bytes.Buffer
				root.SetOut(&stdout)
				root.SetErr(&stderr)
				if err := root.Execute(); err == nil {
					t.Fatal("expected command failure")
				}
				events := []struct {
					name    string
					capture func(*cobra.Command)
				}{
					{DefaultEventName, Capture},
					{EndOfExecutionErrorEventName, func(cmd *cobra.Command) { CaptureError(cmd, stdout.String(), stderr.String()) }},
					{EndOfExecutionEventName, func(cmd *cobra.Command) { CaptureWithEvent(cmd, EndOfExecutionEventName) }},
				}
				for _, event := range events {
					event.capture(command)
					var body []byte
					select {
					case body = <-requests:
					default:
						t.Fatalf("missing %s telemetry request", event.name)
					}
					for _, secret := range []string{token, "refresh-token-secret", "argument-secret"} {
						if bytes.Contains(body, []byte(secret)) {
							t.Errorf("%s telemetry contains secret %q", event.name, secret)
						}
					}
					var payload struct {
						Batch []struct {
							Event      string                 `json:"event"`
							DistinctID string                 `json:"distinct_id"`
							Properties map[string]interface{} `json:"properties"`
						} `json:"batch"`
					}
					if err := json.Unmarshal(body, &payload); err != nil {
						t.Fatal(err)
					}
					if len(payload.Batch) != 1 {
						t.Fatalf("got %d events, want 1", len(payload.Batch))
					}
					capture := payload.Batch[0]
					if capture.Event != event.name || capture.DistinctID != "test-user" {
						t.Errorf("unexpected event identity: %+v", capture)
					}
					want := map[string]string{
						"command": "qovery demo " + name, "flags": "token", "token_type": tokenType,
						"organization": "test-org", "organization_id": "test-org-id",
						"project": "", "project_id": "", "environment": "", "environment_id": "",
						"service": "", "service_id": "", "os": runtime.GOOS, "arch": runtime.GOARCH,
					}
					if event.name == EndOfExecutionErrorEventName {
						want["stdout"] = "demo command output\n"
						want["stderr"] = "demo script failed\n"
					}
					for key, value := range want {
						if capture.Properties[key] != value {
							t.Errorf("property %s = %v, want %q", key, capture.Properties[key], value)
						}
					}
					for key := range capture.Properties {
						// The SDK adds its own properties with the $ prefix.
						if _, ok := want[key]; !ok && !strings.HasPrefix(key, "$") {
							t.Errorf("unexpected telemetry property %q", key)
						}
					}
				}
			})
		}
	}
}

func TestTelemetryOptOutAppliesToAllEvents(t *testing.T) {
	requests := setupTelemetryTest(t, "qov_test-static-token-secret")
	command := &cobra.Command{Use: "qovery"}
	for _, flag := range []string{"false", "FALSE", "FaLsE"} {
		t.Run(flag, func(t *testing.T) {
			t.Setenv("QOVERY_TELEMETRY", flag)
			Capture(command)
			CaptureError(command, "demo command output", "demo script failed")
			CaptureWithEvent(command, EndOfExecutionEventName)
			select {
			case body := <-requests:
				t.Fatalf("telemetry sent despite opt-out: %s", body)
			default:
			}
		})
	}
}
