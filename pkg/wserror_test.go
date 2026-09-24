package pkg

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestIsAgentResponseTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "1011 gateway wait timeout",
			err:  &websocket.CloseError{Code: 1011, Text: "Deadline of 90s exceeded for receiving agent response"},
			want: true,
		},
		{
			name: "1011 shell-agent K8s exec timeout",
			err:  &websocket.CloseError{Code: 1011, Text: "Timed out after 45s while connecting to pod"},
			want: true,
		},
		{
			name: "1011 shell-agent K8s port-forward timeout",
			err:  &websocket.CloseError{Code: 1011, Text: "Timed out after 45s while setting up port forward"},
			want: true,
		},
		{
			name: "1011 shell-agent retry budget exhausted (exec)",
			err:  &websocket.CloseError{Code: 1011, Text: "Retry budget exhausted: only 2s remaining, need at least 45s for K8s exec setup"},
			want: true,
		},
		{
			name: "1011 shell-agent retry budget exhausted (port-forward)",
			err:  &websocket.CloseError{Code: 1011, Text: "Retry budget exhausted: only 1s remaining, need at least 45s for K8s port-forward setup"},
			want: true,
		},
		{
			name: "1011 with different reason falls through to IsInternalServerError",
			err:  &websocket.CloseError{Code: 1011, Text: "some other internal error"},
			want: false,
		},
		{
			name: "wrong close code",
			err:  &websocket.CloseError{Code: 1007, Text: "exceeded for receiving agent response"},
			want: false,
		},
		{
			name: "non-websocket error",
			err:  errors.New("plain network error"),
			want: false,
		},
		{
			name: "wrapped 1011 gateway timeout",
			err:  fmt.Errorf("read failed: %w", &websocket.CloseError{Code: 1011, Text: "Deadline of 90s exceeded for receiving agent response"}),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAgentResponseTimeout(tt.err); got != tt.want {
				t.Errorf("IsAgentResponseTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsAgentResponseTimeoutBeforeIsInternalServerError verifies that a 1011 close error with
// a timeout message matches BOTH IsAgentResponseTimeout (true) and IsInternalServerError (true),
// since timeout is a strict subset of 1011. The test documents why IsAgentResponseTimeout must
// always be checked first in the error-handling chain — otherwise the specific timeout message
// is swallowed by the generic 1011 branch.
func TestIsAgentResponseTimeoutBeforeIsInternalServerError(t *testing.T) {
	for _, text := range []string{
		"Deadline of 90s exceeded for receiving agent response",
		"Timed out after 45s while connecting to pod",
		"Timed out after 45s while setting up port forward",
		"Retry budget exhausted: only 2s remaining, need at least 45s for K8s exec setup",
		"Retry budget exhausted: only 1s remaining, need at least 45s for K8s port-forward setup",
	} {
		err := &websocket.CloseError{Code: 1011, Text: text}
		if !IsAgentResponseTimeout(err) {
			t.Errorf("IsAgentResponseTimeout(%q) = false, want true", text)
		}
		if !IsInternalServerError(err) {
			t.Errorf("IsInternalServerError(%q) = false, want true (timeout is a subset of 1011)", text)
		}
	}
}

func TestIsInternalServerError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"1011 matches", &websocket.CloseError{Code: 1011, Text: "anything"}, true},
		{"1007 does not match", &websocket.CloseError{Code: 1007, Text: ""}, false},
		{"non-websocket error", errors.New("plain error"), false},
		{"wrapped 1011", fmt.Errorf("wrap: %w", &websocket.CloseError{Code: 1011, Text: "x"}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInternalServerError(tt.err); got != tt.want {
				t.Errorf("IsInternalServerError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPermanentCloseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"1007 is permanent", &websocket.CloseError{Code: 1007}, true},
		{"1008 is permanent", &websocket.CloseError{Code: 1008}, true},
		{"1011 is transient", &websocket.CloseError{Code: 1011}, false},
		{"1000 is transient", &websocket.CloseError{Code: 1000}, false},
		{"non-websocket error", errors.New("plain error"), false},
		{"wrapped 1008", fmt.Errorf("wrap: %w", &websocket.CloseError{Code: 1008}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanentCloseError(tt.err); got != tt.want {
				t.Errorf("IsPermanentCloseError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServiceUnavailableMessage(t *testing.T) {
	for _, feature := range []string{"Shell", "Port-forward"} {
		msg := ServiceUnavailableMessage(feature)
		if !strings.HasPrefix(msg, feature) {
			t.Errorf("ServiceUnavailableMessage(%q): expected prefix %q, got: %q", feature, feature, msg)
		}
		if !strings.Contains(msg, "cluster") {
			t.Errorf("ServiceUnavailableMessage(%q): expected 'cluster' in message, got: %q", feature, msg)
		}
		if !strings.Contains(msg, "running") {
			t.Errorf("ServiceUnavailableMessage(%q): expected 'running' in message, got: %q", feature, msg)
		}
		if !strings.HasSuffix(msg, ".") {
			t.Errorf("ServiceUnavailableMessage(%q): expected message to end with '.', got: %q", feature, msg)
		}
	}
}
