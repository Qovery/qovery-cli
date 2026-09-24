package pkg

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"
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

func setupSSHConnection(ctx context.Context) (*exec.Cmd, error) {
	bastionAddress, ok := os.LookupEnv("BASTION_ADDR")
	if !ok {
		log.Error("You must set the bastion address (BASTION_ADDR).")
		os.Exit(1)
	}

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
		return nil, fmt.Errorf("error starting SSH command: %v", err)
	}

	if err := waitForSSHConnection(ctx, "127.0.0.1:1080", 30*time.Second); err != nil {
		if killErr := sshCmd.Process.Kill(); killErr != nil {
			log.Errorf("failed to kill SSH process: %v", killErr)
		}
		return nil, fmt.Errorf("error waiting for SSH connection: %v", err)
	}

	log.Info("SSH connection established successfully")
	if err := os.Setenv("HTTPS_PROXY", "socks5://127.0.0.1:1080"); err != nil {
		if killErr := sshCmd.Process.Kill(); killErr != nil {
			log.Errorf("failed to kill SSH process: %v", killErr)
		}
		return nil, fmt.Errorf("failed to set HTTPS_PROXY: %v", err)
	}

	return sshCmd, nil
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
