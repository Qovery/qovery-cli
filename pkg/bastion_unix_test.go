//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package pkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestBastionSetupFailureKillsAndReapsStartedSSH(t *testing.T) {
	sshPath, pidFile := installFakeSSH(t)
	t.Setenv("PATH", filepath.Dir(sshPath))
	t.Setenv("SSH_PID_FILE", pidFile)
	t.Setenv("HTTPS_PROXY", "existing-proxy")

	_, err := setupSSHConnectionWithAddressAndWait(
		context.Background(),
		"bastion.example",
		func(context.Context, string, time.Duration) error {
			if err := waitForPIDFile(t, pidFile); err != nil {
				return err
			}
			return errors.New("injected tunnel readiness failure")
		},
	)
	if err == nil || !strings.Contains(err.Error(), "injected tunnel readiness failure") {
		t.Fatalf("expected the tunnel readiness error, got %v", err)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "existing-proxy" {
		t.Fatalf("expected proxy environment to remain unchanged, got %q", got)
	}
	assertPIDFileProcessExited(t, pidFile)
}

func TestBastionProxySetupFailureRestoresProxyAndReapsSSH(t *testing.T) {
	sshPath, pidFile := installFakeSSH(t)
	t.Setenv("PATH", filepath.Dir(sshPath))
	t.Setenv("SSH_PID_FILE", pidFile)
	t.Setenv("HTTPS_PROXY", "existing-proxy")

	_, err := setupSSHConnectionWithAddressAndDependencies(
		context.Background(),
		"bastion.example",
		func(context.Context, string, time.Duration) error {
			return waitForPIDFile(t, pidFile)
		},
		func(key, value string) error {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
			return errors.New("injected proxy setup failure")
		},
	)
	if err == nil || !strings.Contains(err.Error(), "injected proxy setup failure") {
		t.Fatalf("expected proxy setup error, got %v", err)
	}
	if got := os.Getenv("HTTPS_PROXY"); got != "existing-proxy" {
		t.Fatalf("expected previous proxy to be restored, got %q", got)
	}
	assertPIDFileProcessExited(t, pidFile)
}

func TestSetBastionConnectionWithErrorCleanupRestoresProxyAndStopsSSH(t *testing.T) {
	sshPath, pidFile := installFakeSSH(t)
	t.Setenv("PATH", filepath.Dir(sshPath))
	t.Setenv("SSH_PID_FILE", pidFile)
	t.Setenv("BASTION_ADDR", "bastion.example")
	t.Setenv("HTTPS_PROXY", "existing-proxy")

	cleanup, err := setBastionConnectionWithError(func(context.Context, string, time.Duration) error {
		return waitForPIDFile(t, pidFile)
	})
	if err != nil {
		t.Fatalf("expected bastion connection, got %v", err)
	}
	if cleanup == nil {
		t.Fatal("expected cleanup callback after successful bastion setup")
	}
	if got, want := os.Getenv("HTTPS_PROXY"), "socks5://127.0.0.1:1080"; got != want {
		t.Fatalf("expected temporary proxy %q, got %q", want, got)
	}

	cleanup()
	if got, want := os.Getenv("HTTPS_PROXY"), "existing-proxy"; got != want {
		t.Fatalf("expected previous proxy %q to be restored, got %q", want, got)
	}
	assertPIDFileProcessExited(t, pidFile)
}

func installFakeSSH(t *testing.T) (sshPath, pidFile string) {
	t.Helper()
	binDir := t.TempDir()
	pidFile = filepath.Join(t.TempDir(), "ssh.pid")
	sshPath = filepath.Join(binDir, "ssh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$$\" > \"$SSH_PID_FILE\"\nexec /bin/sleep 30\n"
	if err := os.WriteFile(sshPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	return sshPath, pidFile
}

func waitForPIDFile(t *testing.T, pidFile string) error {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(pidFile); err == nil {
			return nil
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for SSH PID file %q", pidFile)
		case <-ticker.C:
		}
	}
}

func assertPIDFileProcessExited(t *testing.T, pidFile string) {
	t.Helper()
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(bytes.TrimSpace(pidBytes)))
	if err != nil {
		t.Fatalf("invalid SSH process ID %q: %v", pidBytes, err)
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if err != nil {
			t.Fatalf("checking SSH process %d: %v", pid, err)
		}
		select {
		case <-deadline.C:
			t.Fatalf("SSH process %d remained alive", pid)
		case <-ticker.C:
		}
	}
}
