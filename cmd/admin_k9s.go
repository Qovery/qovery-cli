package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/pkg"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var doNotConnectToBastion bool
var readWriteMode bool

var k9sCmd = &cobra.Command{
	Use:   "k9s",
	Short: "Launch k9s with a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		launchK9s(args)
	},
}

func init() {
	adminCmd.AddCommand(k9sCmd)
	k9sCmd.Flags().BoolVarP(&doNotConnectToBastion, "no-bastion", "n", false, "do not connect to the bastion")
	k9sCmd.Flags().BoolVarP(&readWriteMode, "read-write", "w", false, "run k9s in read-write mode (default is read-only)")
}

func launchK9s(args []string) {
	checkEnv()

	if len(args) == 0 {
		log.Error("You must enter a cluster ID.")
		return
	}

	if !doNotConnectToBastion {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sshCmd, err := setupSSHConnection(ctx)
		if err != nil {
			log.Errorf("Failed to setup SSH connection: %v", err)
			log.Warnf("Connection failure might be due to issues with your SSH configuration. Consider checking and updating your ~/.ssh/known_hosts file to ensure the host is trusted.")
			// continue anyway
		}
		defer cleanupSSHConnection(sshCmd)
	}

	clusterId := args[0]
	kubeconfig := pkg.GetKubeconfigByClusterId(clusterId)
	filePath := utils.WriteInFile(clusterId, "kubeconfig", []byte(kubeconfig))
	os.Setenv("KUBECONFIG", filePath)

	log.Info("Launching k9s.")

	var k9sArgs []string
	// Run in read-only mode by default unless read-write flag is provided
	if !readWriteMode {
		k9sArgs = append(k9sArgs, "--readonly")
		log.Info("Running k9s in read-only mode. Use --read-write flag to enable write operations.")
	} else {
		log.Info("Running k9s in read-write mode.")
	}

	cmd := exec.Command("k9s", k9sArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Error("Can't launch k9s : " + err.Error())
	}

	utils.DeleteFolder(os.Getenv("KUBECONFIG")[0 : len(os.Getenv("KUBECONFIG"))-len("kubeconfig")])
}

func checkEnv() {
	if _, ok := os.LookupEnv("BASTION_ADDR"); !ok {
		log.Error("You must set the bastion address (BASTION_ADDR).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func setupSSHConnection(ctx context.Context) (*exec.Cmd, error) {
	bastionAddress, ok := os.LookupEnv("BASTION_ADDR")
	if !ok {
		log.Error("You must set the bastion address (BASTION_ADDR).")
		os.Exit(1)
	}

	sshArgs := []string{
		"-N", "-D", "1080",
		"-o", "StrictHostKeychecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ServerAliveInterval=10",
		"-o", "ServerAliveCountMax=3",
		"-o", "TCPKeepAlive=yes",
		fmt.Sprintf("root@%s", bastionAddress),
		"-p", "2222",
	}

	sshCmd := exec.CommandContext(ctx, "ssh", sshArgs...)
	if err := sshCmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting SSH command: %v", err)
	}

	if err := waitForSSHConnection(ctx, "localhost:1080", 30*time.Second); err != nil {
		if killErr := sshCmd.Process.Kill(); killErr != nil {
			log.Errorf("failed to kill SSH process: %v", killErr)
		}
		return nil, fmt.Errorf("error waiting for SSH connection: %v", err)
	}

	log.Info("SSH connection established successfully")
	if err := os.Setenv("HTTPS_PROXY", "socks5://localhost:1080"); err != nil {
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
			if conn, err := net.DialTimeout("tcp", address, time.Second); err == nil {
				err := conn.Close()
				if err != nil {
					return err
				}
				return nil
			}
		}
	}
}
