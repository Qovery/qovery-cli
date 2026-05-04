package pkg

import (
	"errors"
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
			name: "1011 with agent timeout reason",
			err:  &websocket.CloseError{Code: 1011, Text: "Deadline of 60s exceeded for receiving agent response"},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAgentResponseTimeout(tt.err); got != tt.want {
				t.Errorf("IsAgentResponseTimeout() = %v, want %v", got, tt.want)
			}
		})
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPermanentCloseError(tt.err); got != tt.want {
				t.Errorf("IsPermanentCloseError() = %v, want %v", got, tt.want)
			}
		})
	}
}
