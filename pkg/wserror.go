package pkg

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// IsPermissionError returns true if the websocket close error indicates
// a permission/authorization failure from the websocket-gateway.
func IsPermissionError(err error) bool {
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return false
	}
	return closeErr.Code == 1007 &&
		strings.Contains(strings.ToLower(closeErr.Text), "permission")
}

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

// PermissionDeniedMessage returns a user-friendly permission denied message for the given feature.
func PermissionDeniedMessage(feature string) string {
	return fmt.Sprintf("Permission denied. Your account does not have access to %s on this service. The minimum required role is Deployer. Contact your Organization admin to update your permissions.", feature)
}

// ServiceUnavailableMessage returns a user-friendly message when the cluster agent is unreachable.
func ServiceUnavailableMessage(feature string) string {
	return fmt.Sprintf("%s is not available. Please verify that the cluster hosting this service is running and healthy.", feature)
}

// PermanentErrorMessage returns the appropriate user-facing message for a permanent websocket error.
func PermanentErrorMessage(err error, feature string) string {
	if IsPermissionError(err) {
		return PermissionDeniedMessage(feature)
	}
	return "Connection rejected by server. Please run 'qovery auth' to re-authenticate, or contact your Organization admin to verify your permissions."
}

// ConnectionFailedMessage returns a user-facing message when the websocket dial fails after retry.
func ConnectionFailedMessage(err error) string {
	return fmt.Sprintf("Error creating websocket connection. Try running 'qovery auth' to re-authenticate: %v", err)
}

// IsAuthDialError returns true if the HTTP response from the websocket handshake
// indicates an authentication or authorization failure (401/403).
// Only these failures justify a token refresh; other dial errors (network, DNS,
// server down) should not trigger a refresh that could corrupt stored credentials.
func IsAuthDialError(resp *http.Response) bool {
	return resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden)
}
