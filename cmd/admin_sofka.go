package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	sofkaNoBastion     bool
	sofkaReadWriteMode bool
	sofkaCmd           = &cobra.Command{
		Use:           "sofka <cluster-id>",
		Short:         "Launch Sofka with a cluster ID",
		Args:          cobra.ExactArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return launchSofka(args[0])
		},
	}
)

func init() {
	adminCmd.AddCommand(sofkaCmd)
	sofkaCmd.Flags().BoolVarP(&sofkaNoBastion, "no-bastion", "n", false, "do not connect to the bastion")
	sofkaCmd.Flags().BoolVarP(&sofkaReadWriteMode, "read-write", "w", false, "run Sofka in read-write mode (default is read-only)")
}

func launchSofka(clusterID string) error {
	return launchSofkaWithDependencies(clusterID, pkg.GetKubeconfigByClusterIdWithError, pkg.SetBastionConnection)
}

func launchSofkaWithDependencies(
	clusterID string,
	fetchKubeconfig func(string, bool) (string, error),
	connectToBastion func() func(),
) (returnErr error) {
	sofkaPath, err := findSofka()
	if err != nil {
		return err
	}

	if !sofkaNoBastion {
		if bastionAddress, ok := os.LookupEnv("BASTION_ADDR"); !ok || strings.TrimSpace(bastionAddress) == "" {
			return fmt.Errorf("you must set a non-empty bastion address (BASTION_ADDR) or pass --no-bastion")
		}

		cleanup := connectToBastion()
		if cleanup != nil {
			defer func() {
				logrus.Info("Cleaning up SSH tunnel...")
				cleanup()
			}()
		}
	}

	kubeconfig, err := fetchKubeconfig(clusterID, sofkaKubeconfigReadOnly(sofkaReadWriteMode))
	if err != nil {
		return fmt.Errorf("failed to retrieve kubeconfig for cluster %s: %w", clusterID, err)
	}

	kubeconfigPath, cleanupKubeconfig, err := createSofkaKubeconfig(kubeconfig)
	if err != nil {
		return err
	}
	defer func() {
		if err := cleanupKubeconfig(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("failed to remove temporary kubeconfig directory: %w", err))
		}
	}()

	if sofkaReadWriteMode {
		logrus.Info("Running Sofka in read-write mode.")
	} else {
		logrus.Info("Running Sofka in read-only mode. Use --read-write to enable write operations.")
	}
	logrus.Info("Launching Sofka.")

	return runSofka(sofkaPath, sofkaArguments(sofkaReadWriteMode), kubeconfigPath)
}

func sofkaArguments(readWrite bool) []string {
	if readWrite {
		return []string{"--write"}
	}
	return []string{"--readonly"}
}

func sofkaKubeconfigReadOnly(readWrite bool) bool {
	return !readWrite
}

func createSofkaKubeconfig(kubeconfig string) (string, func() error, error) {
	if strings.TrimSpace(kubeconfig) == "" {
		return "", nil, errors.New("received an empty kubeconfig")
	}

	tempDir, err := os.MkdirTemp("", "qovery-sofka-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary kubeconfig directory: %w", err)
	}
	cleanup := func() error {
		return os.RemoveAll(tempDir)
	}

	kubeconfigPath := filepath.Join(tempDir, "kubeconfig")
	if err := writeSofkaKubeconfig(kubeconfigPath, kubeconfig); err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("failed to remove temporary kubeconfig directory: %w", cleanupErr))
		}
		return "", nil, err
	}

	return kubeconfigPath, cleanup, nil
}

func writeSofkaKubeconfig(path, kubeconfig string) error {
	if err := os.WriteFile(path, []byte(kubeconfig), 0600); err != nil {
		return fmt.Errorf("failed to write temporary kubeconfig: %w", err)
	}
	return nil
}

func findSofka() (string, error) {
	sofkaPath, err := exec.LookPath("sofka")
	if err != nil {
		return "", fmt.Errorf("sofka executable not found in PATH; install Sofka and try again: %w", err)
	}
	return sofkaPath, nil
}

func runSofka(sofkaPath string, args []string, kubeconfigPath string) error {
	if err := runInteractiveCommandWithSignalForwarding(sofkaPath, args, environmentWithKubeconfig(kubeconfigPath)); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if status := exitError.ExitCode(); status >= 0 {
				return fmt.Errorf("sofka exited with status %d", status)
			}
			return fmt.Errorf("sofka exited unsuccessfully: %w", err)
		}
		return fmt.Errorf("failed to start sofka: %w", err)
	}
	return nil
}
