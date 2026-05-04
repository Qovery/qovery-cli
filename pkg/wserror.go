package pkg

import (
	"errors"
	"strings"

	"github.com/gorilla/websocket"
)

// IsPermanentCloseError returns true if the websocket close error should NOT
// be retried (permission denied, auth/policy violation).
// Transient errors (abnormal closure, going away, internal server error) return false.
func IsPermanentCloseError(err error) bool {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	switch closeErr.Code {
	case 1007: // Invalid frame payload data — used by gateway for permission errors
		return true
	case 1008: // Policy Violation — used for auth/token errors
		return true
	default:
		return false
	}
}

// IsInternalServerError returns true if the websocket close error is code 1011 (Internal Error).
func IsInternalServerError(err error) bool {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	return closeErr.Code == 1011
}

// IsAgentResponseTimeout returns true if the websocket close error indicates
// that K8s operations on the shell-agent side timed out, or that the gateway
// timed out waiting for the agent to respond. All are transient and resolve
// once the pod's Kubernetes exec API is responsive again.
//
// IsAgentResponseTimeout is a strict subset of IsInternalServerError (both match close code 1011).
// Always check IsAgentResponseTimeout before IsInternalServerError, otherwise the specific timeout
// message is swallowed by the generic 1011 branch.
//
// Matched substrings and their sources:
//   - "exceeded for receiving agent response"  — gateway wait (shell_gateway.rs DEFAULT_AGENT_RESPONSE_TIMEOUT)
//   - "while connecting to pod"                — shell-agent K8s exec timeout (shell.rs KUBE_OPERATION_TIMEOUT)
//   - "while setting up port forward"          — shell-agent K8s port-forward timeout (port_forward.rs KUBE_PORT_FORWARD_TIMEOUT)
//   - "Retry budget exhausted"                 — shell-agent retry budget guard (shell.rs / port_forward.rs)
func IsAgentResponseTimeout(err error) bool {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	if closeErr.Code != 1011 {
		return false
	}
	return strings.Contains(closeErr.Text, "exceeded for receiving agent response") ||
		strings.Contains(closeErr.Text, "while connecting to pod") ||
		strings.Contains(closeErr.Text, "while setting up port forward") ||
		strings.Contains(closeErr.Text, "Retry budget exhausted")
}

// ServiceUnavailableMessage returns a user-friendly message when the cluster agent is unreachable.
func ServiceUnavailableMessage(feature string) string {
	return feature + " is not available. Please verify that the cluster hosting this service is running and healthy."
}
