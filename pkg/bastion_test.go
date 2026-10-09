package pkg

import (
	"os"
	"strings"
	"testing"
)

func TestSetBastionConnectionWithErrorRequiresNonemptyAddress(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T)
	}{
		{
			name: "missing address",
			setup: func(t *testing.T) {
				t.Setenv("BASTION_ADDR", "temporary")
				if err := os.Unsetenv("BASTION_ADDR"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:  "empty address",
			setup: func(t *testing.T) { t.Setenv("BASTION_ADDR", "") },
		},
		{
			name:  "whitespace address",
			setup: func(t *testing.T) { t.Setenv("BASTION_ADDR", " \t") },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			cleanup, err := SetBastionConnectionWithError()
			if err == nil || !strings.Contains(err.Error(), "BASTION_ADDR") {
				t.Fatalf("expected a bastion address error, got %v", err)
			}
			if cleanup != nil {
				t.Fatal("expected no cleanup callback when connection setup fails")
			}
		})
	}
}

func TestSetBastionConnectionWithErrorReturnsSSHStartError(t *testing.T) {
	t.Setenv("BASTION_ADDR", "bastion.example")
	t.Setenv("PATH", t.TempDir())

	cleanup, err := SetBastionConnectionWithError()
	if err == nil || !strings.Contains(err.Error(), "starting SSH") {
		t.Fatalf("expected an SSH startup error, got %v", err)
	}
	if cleanup != nil {
		t.Fatal("expected no cleanup callback when SSH fails to start")
	}
}
