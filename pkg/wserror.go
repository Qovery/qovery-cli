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
// the shell-agent timed out while attempting to open a shell session in the pod.
// This is a transient error that resolves once the pod's Kubernetes exec API is responsive again.
//
// The matched substring is produced by:
//   - rust-backend/grpc-gateway/src/lib/shell_gateway.rs  (TIMEOUT_AGENT_FIRST_RESPONSE)
//   - rust-backend/grpc-gateway/src/lib/agent_gateway.rs  (DEFAULT_TIMEOUT_FIRST_MESSAGE)
func IsAgentResponseTimeout(err error) bool {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	return closeErr.Code == 1011 && strings.Contains(closeErr.Text, "exceeded for receiving agent response")
}

// ServiceUnavailableMessage returns a user-friendly message when the cluster agent is unreachable.
func ServiceUnavailableMessage(feature string) string {
	return feature + " is not available. Please verify that the cluster hosting this service is running and healthy."
}
