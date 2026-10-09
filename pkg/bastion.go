package pkg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

func SetBastionConnection() func() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sshCmd, err := setupSSHConnection(ctx)
	if err != nil {
		log.Errorf("Failed to setup SSH connection: %v", err)
		log.Warnf("Connection failure might be due to issues with your SSH configuration. Consider checking and updating your ~/.ssh/known_hosts file to ensure the host is trusted.")
		return func() {}
	}

	return func() {
		cleanupSSHConnection(sshCmd)
	}
}

func SetBastionConnectionWithError() (func(), error) {
	return setBastionConnectionWithError(waitForSSHConnection)
}

func setBastionConnectionWithError(waitForTunnel func(context.Context, string, time.Duration) error) (func(), error) {
	bastionAddress, ok := os.LookupEnv("BASTION_ADDR")
	if !ok || strings.TrimSpace(bastionAddress) == "" {
		return nil, errors.New("you must set a non-empty bastion address (BASTION_ADDR)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	previousProxy, hadPreviousProxy := os.LookupEnv("HTTPS_PROXY")
	sshCmd, err := setupSSHConnectionWithAddressAndWait(ctx, bastionAddress, waitForTunnel)
	if err != nil {
		return nil, err
	}

	return func() {
		cleanupSSHConnection(sshCmd)
		if err := restoreHTTPSProxy(previousProxy, hadPreviousProxy); err != nil {
			log.Errorf("Failed to restore HTTPS_PROXY: %v", err)
		}
	}, nil
}

func setupSSHConnection(ctx context.Context) (*exec.Cmd, error) {
	bastionAddress, ok := os.LookupEnv("BASTION_ADDR")
	if !ok {
		log.Error("You must set the bastion address (BASTION_ADDR).")
		os.Exit(1)
	}
	return setupSSHConnectionWithAddressAndWait(ctx, bastionAddress, waitForSSHConnection)
}

func setupSSHConnectionWithAddressAndWait(ctx context.Context, bastionAddress string, waitForTunnel func(context.Context, string, time.Duration) error) (*exec.Cmd, error) {
	return setupSSHConnectionWithAddressAndDependencies(ctx, bastionAddress, waitForTunnel, os.Setenv)
}

func setupSSHConnectionWithAddressAndDependencies(
	ctx context.Context,
	bastionAddress string,
	waitForTunnel func(context.Context, string, time.Duration) error,
	setProxy func(string, string) error,
) (*exec.Cmd, error) {
	sshArgs := []string{
		"-N", "-D", "127.0.0.1:1080",
		"-p", "2222",
		"-4",
		"-o", "StrictHostKeychecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		fmt.Sprintf("root@%s", bastionAddress),
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	if err := sshCmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting SSH command: %w", err)
	}

	if err := waitForTunnel(ctx, "127.0.0.1:1080", 30*time.Second); err != nil {
		cleanupErr := killAndWaitSSHProcess(sshCmd)
		return nil, errors.Join(fmt.Errorf("error waiting for SSH connection: %w", err), cleanupErr)
	}

	log.Info("SSH connection established successfully")
	previousProxy, hadPreviousProxy := os.LookupEnv("HTTPS_PROXY")
	if err := setProxy("HTTPS_PROXY", "socks5://127.0.0.1:1080"); err != nil {
		cleanupErr := killAndWaitSSHProcess(sshCmd)
		restoreErr := restoreHTTPSProxy(previousProxy, hadPreviousProxy)
		return nil, errors.Join(fmt.Errorf("failed to set HTTPS_PROXY: %w", err), cleanupErr, restoreErr)
	}

	return sshCmd, nil
}

func killAndWaitSSHProcess(sshCmd *exec.Cmd) error {
	if sshCmd == nil || sshCmd.Process == nil {
		return nil
	}

	killErr := sshCmd.Process.Kill()
	waitErr := sshCmd.Wait()
	var cleanupErr error
	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("failed to kill SSH process: %w", killErr))
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("failed to wait for SSH process: %w", waitErr))
	}
	return cleanupErr
}

func restoreHTTPSProxy(previousProxy string, hadPreviousProxy bool) error {
	if hadPreviousProxy {
		return os.Setenv("HTTPS_PROXY", previousProxy)
	}
	return os.Unsetenv("HTTPS_PROXY")
}

func cleanupSSHConnection(sshCmd *exec.Cmd) {
	if sshCmd != nil && sshCmd.Process != nil {
		log.Info("Terminating SSH process...")
		if err := sshCmd.Process.Signal(syscall.SIGTERM); err != nil {
			log.Errorf("Failed to terminate SSH process: %v", err)
			if err := sshCmd.Process.Kill(); err != nil {
				log.Errorf("Failed to kill SSH process: %v", err)
			}
		}
		_, _ = sshCmd.Process.Wait()
		log.Info("SSH process terminated")
	}

	if err := os.Unsetenv("HTTPS_PROXY"); err != nil {
		log.Errorf("Failed to unset HTTPS_PROXY: %v", err)
	} else {
		log.Info("HTTPS_PROXY has been unset")
	}
}

func waitForSSHConnection(ctx context.Context, address string, timeout time.Duration) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for SSH connection")
		case <-ticker.C:
			if conn, err := net.DialTimeout("tcp4", address, time.Second); err == nil {
				err := conn.Close()
				if err != nil {
					return err
				}
				return nil
			}
		}
	}
}
