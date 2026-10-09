package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestSofkaMode(t *testing.T) {
	tests := []struct {
		name               string
		readWrite          bool
		wantReadOnlyConfig bool
		wantArgs           []string
	}{
		{
			name:               "read-only by default",
			wantReadOnlyConfig: true,
			wantArgs:           []string{"--readonly"},
		},
		{
			name:      "read-write",
			readWrite: true,
			wantArgs:  []string{"--write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sofkaArguments(tt.readWrite); !reflect.DeepEqual(got, tt.wantArgs) {
				t.Fatalf("expected arguments %v, got %v", tt.wantArgs, got)
			}
			if got := sofkaKubeconfigReadOnly(tt.readWrite); got != tt.wantReadOnlyConfig {
				t.Fatalf("expected read-only kubeconfig %t, got %t", tt.wantReadOnlyConfig, got)
			}
		})
	}
}

func TestLaunchSofkaUsesPrivateKubeconfigAndCleansItUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executable uses a shell script")
	}

	for _, tt := range []struct {
		name                   string
		readWrite              bool
		wantReadOnlyKubeconfig bool
		wantArgs               string
	}{
		{
			name:                   "read-only",
			wantReadOnlyKubeconfig: true,
			wantArgs:               "--readonly",
		},
		{
			name:      "read-write",
			readWrite: true,
			wantArgs:  "--write",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			binDir, argsFile, kubeconfigPathFile, kubeconfigContentFile := installFakeSofka(t, 0)
			t.Setenv("PATH", binDir)
			t.Setenv("SOFKA_ARGS_FILE", argsFile)
			t.Setenv("SOFKA_KUBECONFIG_FILE", kubeconfigPathFile)
			t.Setenv("SOFKA_CONTENT_FILE", kubeconfigContentFile)
			t.Setenv("KUBECONFIG", "previous-kubeconfig")
			t.Setenv("BASTION_ADDR", "")
			setSofkaFlags(t, true, tt.readWrite)

			var fetchedClusterID string
			var fetchedReadOnly bool
			fetchKubeconfig := func(clusterID string, readOnly bool) (string, error) {
				fetchedClusterID = clusterID
				fetchedReadOnly = readOnly
				return "apiVersion: v1\n", nil
			}
			connectToBastion := func() func() {
				t.Fatal("no-bastion must skip tunnel setup")
				return func() {}
			}

			if err := launchSofkaWithDependencies("cluster-id", fetchKubeconfig, connectToBastion); err != nil {
				t.Fatalf("expected Sofka to launch, got %v", err)
			}
			if fetchedClusterID != "cluster-id" || fetchedReadOnly != tt.wantReadOnlyKubeconfig {
				t.Fatalf("fetched kubeconfig for (%q, readOnly=%t), want (cluster-id, readOnly=%t)", fetchedClusterID, fetchedReadOnly, tt.wantReadOnlyKubeconfig)
			}
			if got := os.Getenv("KUBECONFIG"); got != "previous-kubeconfig" {
				t.Fatalf("expected KUBECONFIG to be restored, got %q", got)
			}

			args, err := os.ReadFile(argsFile)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.TrimSpace(string(args)); got != tt.wantArgs {
				t.Fatalf("expected Sofka arguments %q, got %q", tt.wantArgs, got)
			}

			kubeconfigPathBytes, err := os.ReadFile(kubeconfigPathFile)
			if err != nil {
				t.Fatal(err)
			}
			kubeconfigPath := strings.TrimSpace(string(kubeconfigPathBytes))
			if _, err := os.Stat(kubeconfigPath); !os.IsNotExist(err) {
				t.Fatalf("expected temporary kubeconfig %q to be removed, stat error: %v", kubeconfigPath, err)
			}

			kubeconfigContent, err := os.ReadFile(kubeconfigContentFile)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := string(kubeconfigContent), "apiVersion: v1\n"; got != want {
				t.Fatalf("expected kubeconfig contents %q, got %q", want, got)
			}
		})
	}
}

func TestSofkaKubeconfigUsesPrivateDirectoryAndFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}

	path, cleanup, err := createSofkaKubeconfig("apiVersion: v1\n")
	if err != nil {
		t.Fatalf("expected kubeconfig file, got %v", err)
	}
	t.Cleanup(func() { _ = cleanup() })

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fileInfo.Mode().Perm(), os.FileMode(0600); got != want {
		t.Fatalf("expected file permissions %04o, got %04o", want, got)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dirInfo.Mode().Perm(), os.FileMode(0700); got != want {
		t.Fatalf("expected directory permissions %04o, got %04o", want, got)
	}

	if err := cleanup(); err != nil {
		t.Fatalf("expected temporary directory cleanup, got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected kubeconfig file to be removed, stat error: %v", err)
	}
}

func TestSofkaKubeconfigRejectsEmptyContent(t *testing.T) {
	if _, _, err := createSofkaKubeconfig("\n \t"); err == nil || !strings.Contains(err.Error(), "empty kubeconfig") {
		t.Fatalf("expected an explicit empty kubeconfig error, got %v", err)
	}
}

func TestSofkaKubeconfigWriteFailureIsReturned(t *testing.T) {
	targetIsDirectory := filepath.Join(t.TempDir(), "kubeconfig")
	if err := os.Mkdir(targetIsDirectory, 0700); err != nil {
		t.Fatal(err)
	}

	err := writeSofkaKubeconfig(targetIsDirectory, "apiVersion: v1\n")
	if err == nil || !strings.Contains(err.Error(), "failed to write temporary kubeconfig") {
		t.Fatalf("expected a kubeconfig write error, got %v", err)
	}
}

func TestLaunchSofkaCleansUpAfterKubeconfigFetchFailure(t *testing.T) {
	binDir, _, _, _ := installFakeSofka(t, 0)
	t.Setenv("PATH", binDir)
	t.Setenv("BASTION_ADDR", "bastion.example")
	setSofkaFlags(t, false, false)

	var tunnelStarted, tunnelCleaned bool
	err := launchSofkaWithDependencies(
		"cluster-id",
		func(string, bool) (string, error) {
			return "", errors.New("kubeconfig API unavailable")
		},
		func() func() {
			tunnelStarted = true
			return func() { tunnelCleaned = true }
		},
	)
	if err == nil || !strings.Contains(err.Error(), "kubeconfig API unavailable") {
		t.Fatalf("expected kubeconfig fetch error, got %v", err)
	}
	if !tunnelStarted || !tunnelCleaned {
		t.Fatalf("expected tunnel cleanup after fetch failure, started=%t cleaned=%t", tunnelStarted, tunnelCleaned)
	}
}

func TestLaunchSofkaCleansUpAfterSofkaExitsWithError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executable uses a shell script")
	}

	binDir := t.TempDir()
	sofkaPath := filepath.Join(binDir, "sofka")
	if err := os.WriteFile(sofkaPath, []byte("#!/bin/sh\nprintf '%s\\n' \"$KUBECONFIG\" > \"$SOFKA_KUBECONFIG_FILE\"\nexit 7\n"), 0755); err != nil {
		t.Fatal(err)
	}
	kubeconfigPathFile := filepath.Join(t.TempDir(), "kubeconfig-path")
	t.Setenv("PATH", binDir)
	t.Setenv("SOFKA_KUBECONFIG_FILE", kubeconfigPathFile)
	t.Setenv("KUBECONFIG", "previous-kubeconfig")
	setSofkaFlags(t, true, false)

	err := launchSofkaWithDependencies(
		"cluster-id",
		func(string, bool) (string, error) { return "apiVersion: v1\n", nil },
		func() func() { return func() {} },
	)
	if err == nil || !strings.Contains(err.Error(), "sofka exited with status 7") {
		t.Fatalf("expected child exit status error, got %v", err)
	}
	if got := os.Getenv("KUBECONFIG"); got != "previous-kubeconfig" {
		t.Fatalf("expected KUBECONFIG to be restored, got %q", got)
	}

	kubeconfigPathBytes, err := os.ReadFile(kubeconfigPathFile)
	if err != nil {
		t.Fatal(err)
	}
	kubeconfigPath := strings.TrimSpace(string(kubeconfigPathBytes))
	if _, err := os.Stat(kubeconfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected temporary kubeconfig %q to be removed, stat error: %v", kubeconfigPath, err)
	}
}

func TestRunSofkaDistinguishesChildExitFromLaunchFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test executables use shell scripts")
	}

	binDir, _, _, _ := installFakeSofka(t, 7)
	t.Setenv("PATH", binDir)
	sofkaPath, err := findSofka()
	if err != nil {
		t.Fatalf("expected to find Sofka, got %v", err)
	}
	if err := runSofka(sofkaPath, nil); err == nil || !strings.Contains(err.Error(), "sofka exited with status 7") || strings.Contains(err.Error(), "failed to launch") {
		t.Fatalf("expected a child exit status error, got %v", err)
	}

	failedLaunchPath := filepath.Join(t.TempDir(), "sofka")
	if err := os.WriteFile(failedLaunchPath, []byte("#!/missing/interpreter\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := runSofka(failedLaunchPath, nil); err == nil || !strings.Contains(err.Error(), "failed to start sofka") {
		t.Fatalf("expected an executable launch error, got %v", err)
	}
}

func TestFindSofkaReturnsClearErrorWhenExecutableIsMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := findSofka()
	if err == nil || !strings.Contains(err.Error(), "sofka executable not found in PATH") {
		t.Fatalf("expected a clear missing executable error, got %v", err)
	}
}

func TestSofkaCommandErrorsUseNonzeroExitStatusOnlyForSofka(t *testing.T) {
	if got := commandErrorExitCode(sofkaCmd); got != 1 {
		t.Fatalf("expected Sofka errors to exit with status 1, got %d", got)
	}
	if got := commandErrorExitCode(rootCmd); got != 0 {
		t.Fatalf("expected other command behavior to remain unchanged, got status %d", got)
	}
}

func setSofkaFlags(t *testing.T, noBastion, readWrite bool) {
	t.Helper()
	previousNoBastion := sofkaNoBastion
	previousReadWrite := sofkaReadWriteMode
	sofkaNoBastion = noBastion
	sofkaReadWriteMode = readWrite
	t.Cleanup(func() {
		sofkaNoBastion = previousNoBastion
		sofkaReadWriteMode = previousReadWrite
	})
}

func installFakeSofka(t *testing.T, exitCode int) (binDir, argsFile, kubeconfigPathFile, kubeconfigContentFile string) {
	t.Helper()
	binDir = t.TempDir()
	recordDir := t.TempDir()
	argsFile = filepath.Join(recordDir, "args")
	kubeconfigPathFile = filepath.Join(recordDir, "kubeconfig-path")
	kubeconfigContentFile = filepath.Join(recordDir, "kubeconfig-content")
	sofkaPath := filepath.Join(binDir, "sofka")
	script := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	if exitCode == 0 {
		script = "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$SOFKA_ARGS_FILE\"\nprintf '%s\\n' \"$KUBECONFIG\" > \"$SOFKA_KUBECONFIG_FILE\"\nIFS= read -r kubeconfig_content < \"$KUBECONFIG\"\nprintf '%s\\n' \"$kubeconfig_content\" > \"$SOFKA_CONTENT_FILE\"\n"
	}
	if err := os.WriteFile(sofkaPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return binDir, argsFile, kubeconfigPathFile, kubeconfigContentFile
}
