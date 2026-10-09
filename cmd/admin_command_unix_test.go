//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunCommandForwardsSignalsToChild(t *testing.T) {
	for _, signal := range interactiveCommandSignals() {
		t.Run(signal.String(), func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			commandPath := filepath.Join(t.TempDir(), "wait-for-signal")
			script := "#!/bin/sh\nprintf '%s\\n' \"$$\" > \"$SIGNAL_CHILD_PID_FILE\"\nexec /bin/sleep 30\n"
			if err := os.WriteFile(commandPath, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}

			command := exec.Command(commandPath)
			command.Env = append(os.Environ(), "SIGNAL_CHILD_PID_FILE="+pidFile)
			signals := make(chan os.Signal, 1)
			result := make(chan error, 1)
			go func() { result <- runCommandWithSignalForwarding(command, signals) }()

			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			poll := time.NewTicker(10 * time.Millisecond)
			defer poll.Stop()
		forWait:
			for {
				if _, err := os.Stat(pidFile); err == nil {
					break forWait
				}
				select {
				case err := <-result:
					t.Fatalf("child exited before receiving a signal: %v", err)
				case <-deadline.C:
					t.Fatal("timed out waiting for child to start")
				case <-poll.C:
				}
			}

			signals <- signal
			select {
			case err := <-result:
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("expected child to exit due to %s, got %v", signal, err)
				}
				waitStatus, ok := exitErr.Sys().(syscall.WaitStatus)
				if !ok || !waitStatus.Signaled() || waitStatus.Signal() != signal.(syscall.Signal) {
					t.Fatalf("expected child to receive %s, got %v", signal, exitErr.Sys())
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("child did not exit after %s was forwarded", signal)
			}
		})
	}
}

func TestInteractiveCommandSignalsIncludeCommonTerminationSignals(t *testing.T) {
	var got []string
	for _, signal := range interactiveCommandSignals() {
		got = append(got, signal.String())
	}
	for _, want := range []string{"interrupt", "terminated", "hangup"} {
		found := false
		for _, value := range got {
			if strings.EqualFold(value, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected signal list %v to include %q", got, want)
		}
	}
}

func TestLaunchSofkaCleansUpAfterSIGTERM(t *testing.T) {
	binDir := t.TempDir()
	kubeconfigPathFile := filepath.Join(t.TempDir(), "kubeconfig-path")
	sofkaPath := filepath.Join(binDir, "sofka")
	script := "#!/bin/sh\nprintf '%s\\n' \"$KUBECONFIG\" > \"$SOFKA_SIGNAL_KUBECONFIG_FILE\"\nkill -TERM \"$PPID\"\nexec /bin/sleep 5\n"
	if err := os.WriteFile(sofkaPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	command := exec.Command(os.Args[0], "-test.run=^TestSofkaSignalCleanupHarness$")
	command.Env = append(os.Environ(),
		"QOVERY_SOFKA_SIGNAL_HELPER=1",
		"QOVERY_SOFKA_SIGNAL_BIN_DIR="+binDir,
		"SOFKA_SIGNAL_KUBECONFIG_FILE="+kubeconfigPathFile,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("signal cleanup helper failed: %v\n%s", err, output)
	}
}

func TestSofkaSignalCleanupHarness(t *testing.T) {
	if os.Getenv("QOVERY_SOFKA_SIGNAL_HELPER") != "1" {
		return
	}

	t.Setenv("PATH", os.Getenv("QOVERY_SOFKA_SIGNAL_BIN_DIR"))
	t.Setenv("BASTION_ADDR", "bastion.example")
	setSofkaFlags(t, false, false)
	tunnelCleaned := false
	err := launchSofkaWithDependencies(
		"cluster-id",
		func(string, bool) (string, error) { return "apiVersion: v1\n", nil },
		func() (func(), error) { return func() { tunnelCleaned = true }, nil },
	)
	if err == nil || !strings.Contains(err.Error(), "sofka exited unsuccessfully") {
		t.Fatalf("expected Sofka to stop after SIGTERM, got %v", err)
	}
	if !tunnelCleaned {
		t.Fatal("expected bastion tunnel cleanup after SIGTERM")
	}

	kubeconfigPathBytes, err := os.ReadFile(os.Getenv("SOFKA_SIGNAL_KUBECONFIG_FILE"))
	if err != nil {
		t.Fatal(err)
	}
	kubeconfigPath := strings.TrimSpace(string(kubeconfigPathBytes))
	if _, err := os.Stat(kubeconfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected temporary kubeconfig %q to be removed, stat error: %v", kubeconfigPath, err)
	}
}
