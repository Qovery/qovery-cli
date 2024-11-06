package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var doNotConnectToBastion bool

var k9sCmd = &cobra.Command{
	Use:   "k9s",
	Short: "Launch k9s with a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		launchK9s(args)
	},
}

func init() {
	adminCmd.AddCommand(k9sCmd)
	k9sCmd.Flags().BoolVarP(&doNotConnectToBastion, "no-bastion", "", false, "do not connect to the bastion")
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
	vars, err := pkg.GetVarsByClusterId(clusterId)
	if len(vars) == 0 || err != nil {
		return
	}

	for _, variable := range vars {
		os.Setenv(variable.Key, variable.Value)

		// Generate temporary file + ENV for GCP auth
		// https://serverfault.com/questions/848580/how-to-use-google-application-credentials-with-gcloud-on-a-server
		if variable.Key == "GOOGLE_CREDENTIALS" {
			googleCredentialsFile, err := os.CreateTemp("", "sample")
			if err != nil {
				log.Error("Can't create google credentials file : " + err.Error())
			}
			defer os.Remove(googleCredentialsFile.Name())

			_, err = googleCredentialsFile.WriteString(variable.Value)
			if err != nil {
				log.Error("Can't create google credentials file : " + err.Error())
			}

			os.Setenv("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", googleCredentialsFile.Name())
		}
	}
	utils.GenerateExportEnvVarsScript(vars, args[0])

	log.Info("Launching k9s.")
	cmd := exec.Command("k9s")
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		log.Error("Can't launch k9s : " + err.Error())
	}

	utils.DeleteFolder(os.Getenv("KUBECONFIG")[0 : len(os.Getenv("KUBECONFIG"))-len("kubeconfig")])
}

func checkEnv() {
	if _, ok := os.LookupEnv("VAULT_ADDR"); !ok {
		log.Error("You must set vault address env variable (VAULT_ADDR).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if _, ok := os.LookupEnv("VAULT_TOKEN"); !ok {
		log.Error("You must set vault token env variable (VAULT_TOKEN).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

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
		err := sshCmd.Process.Kill()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("error waiting for SSH connection: %v", err)
	}

	log.Info("SSH connection established successfully")
	if err := os.Setenv("HTTPS_PROXY", "socks5://localhost:1080"); err != nil {
		err := sshCmd.Process.Kill()
		if err != nil {
			return nil, err
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
