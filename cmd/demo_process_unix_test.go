//go:build unix

package cmd

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestPrepareDemoTokenFileCleanup(t *testing.T) {
	dir := t.TempDir()
	stalePath := filepath.Join(dir, "qovery-token-stale")
	logPath := filepath.Join(dir, "qovery-demo.log")
	for _, path := range []string{stalePath, logPath} {
		if err := os.WriteFile(path, []byte("test data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	path, cleanup, err := prepareDemoTokenFile(dir, "Bearer", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("stale token was not removed: %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("demo log must be preserved: %v", err)
	}

	// A second invocation must not delete a token that is still in use.
	_, secondCleanup, err := prepareDemoTokenFile(dir, "Token", "qov_other-token")
	if err == nil {
		secondCleanup()
		t.Fatal("expected the active demo to retain the directory lock")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "Authorization: Bearer test-token\n" {
		t.Fatalf("active demo token was changed or removed: %q, %v", data, err)
	}
	cleanup()
	cleanup() // Cleanup is also deferred by the caller.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("token was not removed: %v", err)
	}
	_, nextCleanup, err := prepareDemoTokenFile(dir, "Token", "qov_next-token")
	if err != nil {
		t.Fatalf("directory lock was not released: %v", err)
	}
	nextCleanup()
}

func TestDemoTokenCleanupOnSignal(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL} {
		t.Run(sig.String(), func(t *testing.T) {
			dir := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			helper := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestDemoTokenSignalHelper$")
			helper.WaitDelay = time.Second
			helper.Env = append(os.Environ(), "QOVERY_TEST_DEMO_TOKEN_DIR="+dir)
			var stderr bytes.Buffer
			helper.Stderr = &stderr
			stdout, err := helper.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := helper.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = helper.Process.Kill() }()
			// The child announces readiness only after the token exists and the
			// shell process group has started. Never signal the test runner.
			scanner := bufio.NewScanner(stdout)
			if !scanner.Scan() {
				_ = helper.Wait()
				t.Fatalf("helper did not start: %v, %s", scanner.Err(), stderr.String())
			}
			pidText, found := strings.CutPrefix(scanner.Text(), "ready ")
			if !found {
				t.Fatalf("unexpected helper output: %q", scanner.Text())
			}
			childPID, err := strconv.Atoi(pidText)
			if err != nil || childPID <= 0 {
				t.Fatalf("invalid child PID %q: %v", pidText, err)
			}
			// SIGKILL cannot be handled by the helper, so its child must be
			// stopped by the parent test in that case.
			defer func() { _ = unix.Kill(-childPID, unix.SIGKILL) }()
			paths, err := filepath.Glob(filepath.Join(dir, "qovery-token-*"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("expected one active token file: %v, %v", paths, err)
			}
			if err := helper.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			err = helper.Wait()
			if ctx.Err() != nil {
				t.Fatalf("helper did not terminate after %v: %s", sig, stderr.String())
			}
			if sig == syscall.SIGKILL {
				if err == nil {
					t.Fatal("expected helper to be killed")
				}
				if _, err := os.Stat(paths[0]); err != nil {
					t.Fatalf("expected leftover token after SIGKILL: %v", err)
				}
				_, cleanup, err := prepareDemoTokenFile(dir, "Bearer", "next-token")
				if err != nil {
					t.Fatalf("could not recover after SIGKILL: %v", err)
				}
				cleanup()
			} else if err != nil {
				t.Fatalf("helper did not cleanly handle %v: %v, %s", sig, err, stderr.String())
			}
			if _, err := os.Stat(paths[0]); !os.IsNotExist(err) {
				t.Fatalf("token file remains after %v: %v", sig, err)
			}
		})
	}
}

// Run in a separate process so real signals cannot interrupt other tests.
func TestDemoTokenSignalHelper(t *testing.T) {
	dir := os.Getenv("QOVERY_TEST_DEMO_TOKEN_DIR")
	if dir == "" {
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	_, cleanup, err := prepareDemoTokenFile(dir, "Bearer", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	command := exec.CommandContext(ctx, "/bin/sh", "-c", `printf 'ready %s\n' "$$"; exec sleep 60`)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); ctx.Err() == nil || err == nil {
		t.Fatalf("expected command to stop on cancellation: %v", err)
	}
}
