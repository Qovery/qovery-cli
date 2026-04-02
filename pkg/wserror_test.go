package pkg

import (
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestIsPermissionError_WithPermissionMessage(t *testing.T) {
	err := &websocket.CloseError{
		Code: 1007,
		Text: "Invalid permission: Not authorized to websocket-gateway SHELL_EXEC. If you need access, ask your Organization admin to assign",
	}
	assert.True(t, IsPermissionError(err))
}

func TestIsPermissionError_WithNormalClosure(t *testing.T) {
	err := &websocket.CloseError{
		Code: websocket.CloseNormalClosure,
		Text: "bye",
	}
	assert.False(t, IsPermissionError(err))
}

func TestIsPermissionError_WithNilError(t *testing.T) {
	assert.False(t, IsPermissionError(nil))
}

func TestIsPermissionError_WithNonWebsocketError(t *testing.T) {
	assert.False(t, IsPermissionError(assert.AnError))
}

func TestIsPermanentCloseError_WithPermission(t *testing.T) {
	err := &websocket.CloseError{
		Code: 1007,
		Text: "Invalid permission: Not authorized",
	}
	assert.True(t, IsPermanentCloseError(err))
}

func TestIsPermanentCloseError_WithAuthExpired(t *testing.T) {
	err := &websocket.CloseError{
		Code: 1008,
		Text: "token expired",
	}
	assert.True(t, IsPermanentCloseError(err))
}

func TestIsPermanentCloseError_WithInternalError(t *testing.T) {
	// 1011 is transient (shell-agent may come up during cluster startup)
	err := &websocket.CloseError{
		Code: 1011,
		Text: "No shell-agent listening for this cluster ec865376-15ca-4c3d-9eae-ccd6c089d1e0",
	}
	assert.False(t, IsPermanentCloseError(err))
}

func TestIsInternalServerError_WithCode1011(t *testing.T) {
	err := &websocket.CloseError{
		Code: 1011,
		Text: "No shell-agent listening for this cluster",
	}
	assert.True(t, IsInternalServerError(err))
}

func TestIsInternalServerError_WithOtherCode(t *testing.T) {
	err := &websocket.CloseError{
		Code: 1007,
		Text: "Invalid permission",
	}
	assert.False(t, IsInternalServerError(err))
}

func TestIsInternalServerError_WithNilError(t *testing.T) {
	assert.False(t, IsInternalServerError(nil))
}

func TestPermanentErrorMessage_Permission(t *testing.T) {
	err := &websocket.CloseError{Code: 1007, Text: "Invalid permission: Not authorized"}
	msg := PermanentErrorMessage(err, "Shell")
	assert.Contains(t, msg, "Permission denied")
	assert.Contains(t, msg, "Shell")
	assert.Contains(t, msg, "Deployer")
}

func TestPermanentErrorMessage_Fallback(t *testing.T) {
	err := &websocket.CloseError{Code: 1008, Text: "token expired"}
	msg := PermanentErrorMessage(err, "Shell")
	assert.Contains(t, msg, "qovery auth")
}

func TestIsPermanentCloseError_WithTransientError(t *testing.T) {
	err := &websocket.CloseError{
		Code: 1006,
		Text: "connection reset",
	}
	assert.False(t, IsPermanentCloseError(err))
}
