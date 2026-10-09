package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSofkaArguments(t *testing.T) {
	tests := []struct {
		name      string
		readWrite bool
		want      []string
	}{
		{
			name: "read-only by default",
			want: []string{"--readonly"},
		},
		{
			name:      "read-write",
			readWrite: true,
			want:      []string{"--write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sofkaArguments(tt.readWrite); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("expected arguments %v, got %v", tt.want, got)
			}
		})
	}
}

func TestRunSofkaPassesArgumentsAndKubeconfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executable uses a shell script")
	}

	binDir := t.TempDir()
	argsFile := filepath.Join(t.TempDir(), "args")
	kubeconfigFile := filepath.Join(t.TempDir(), "kubeconfig")
	sofkaPath := filepath.Join(binDir, "sofka")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SOFKA_ARGS_FILE\"\nprintf '%s' \"$KUBECONFIG\" > \"$SOFKA_KUBECONFIG_FILE\"\n"
	if err := os.WriteFile(sofkaPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", binDir)
	t.Setenv("SOFKA_ARGS_FILE", argsFile)
	t.Setenv("SOFKA_KUBECONFIG_FILE", kubeconfigFile)
	t.Setenv("KUBECONFIG", "/tmp/qovery_test/kubeconfig")

	sofkaPath, err := findSofka()
	if err != nil {
		t.Fatalf("expected to find Sofka, got %v", err)
	}
	if err := runSofka(sofkaPath, sofkaArguments(true)); err != nil {
		t.Fatalf("expected Sofka to run, got %v", err)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(args)), "--write"; got != want {
		t.Fatalf("expected Sofka arguments %q, got %q", want, got)
	}

	kubeconfig, err := os.ReadFile(kubeconfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(kubeconfig), "/tmp/qovery_test/kubeconfig"; got != want {
		t.Fatalf("expected KUBECONFIG %q, got %q", want, got)
	}
}

func TestRunSofkaReturnsClearErrorWhenExecutableIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := findSofka()
	if err == nil || !strings.Contains(err.Error(), "sofka executable not found in PATH") {
		t.Fatalf("expected a clear missing executable error, got %v", err)
	}
}

func TestRunSofkaReturnsLaunchFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executable uses a shell script")
	}

	binDir := t.TempDir()
	sofkaPath := filepath.Join(binDir, "sofka")
	if err := os.WriteFile(sofkaPath, []byte("#!/bin/sh\nexit 7\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	sofkaPath, err := findSofka()
	if err != nil {
		t.Fatalf("expected to find Sofka, got %v", err)
	}
	err = runSofka(sofkaPath, nil)
	if err == nil || !strings.Contains(err.Error(), "failed to launch sofka") || !strings.Contains(err.Error(), "exit status 7") {
		t.Fatalf("expected a clear launch failure, got %v", err)
	}
}
