package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	sofkaNoBastion     bool
	sofkaReadWriteMode bool
	sofkaCmd           = &cobra.Command{
		Use:   "sofka <cluster-id>",
		Short: "Launch Sofka with a cluster ID",
		Args:  cobra.ExactArgs(1),
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
	sofkaPath, err := findSofka()
	if err != nil {
		return err
	}

	if !sofkaNoBastion {
		if _, ok := os.LookupEnv("BASTION_ADDR"); !ok {
			return fmt.Errorf("you must set the bastion address (BASTION_ADDR) or pass --no-bastion")
		}

		cleanup := pkg.SetBastionConnection()
		defer func() {
			log.Info("Cleaning up SSH tunnel...")
			cleanup()
		}()
	}

	kubeconfig := pkg.GetKubeconfigByClusterId(clusterID, false)
	kubeconfigDir := utils.GetFullPath(clusterID)
	kubeconfigPath := utils.WriteInFile(clusterID, "kubeconfig", []byte(kubeconfig))
	defer utils.DeleteFolder(kubeconfigDir)
	if kubeconfigPath == "" {
		return fmt.Errorf("failed to write kubeconfig for cluster %s", clusterID)
	}

	previousKubeconfig, hadPreviousKubeconfig := os.LookupEnv("KUBECONFIG")
	if err := os.Setenv("KUBECONFIG", kubeconfigPath); err != nil {
		return fmt.Errorf("failed to set KUBECONFIG: %w", err)
	}
	defer func() {
		if hadPreviousKubeconfig {
			if err := os.Setenv("KUBECONFIG", previousKubeconfig); err != nil {
				log.Warnf("Failed to restore KUBECONFIG: %v", err)
			}
		} else if err := os.Unsetenv("KUBECONFIG"); err != nil {
			log.Warnf("Failed to clear KUBECONFIG: %v", err)
		}
	}()

	if sofkaReadWriteMode {
		log.Info("Running Sofka in read-write mode.")
	} else {
		log.Info("Running Sofka in read-only mode. Use --read-write to enable write operations.")
	}
	log.Info("Launching Sofka.")

	return runSofka(sofkaPath, sofkaArguments(sofkaReadWriteMode))
}

func sofkaArguments(readWrite bool) []string {
	if readWrite {
		return []string{"--write"}
	}
	return []string{"--readonly"}
}

func findSofka() (string, error) {
	sofkaPath, err := exec.LookPath("sofka")
	if err != nil {
		return "", fmt.Errorf("sofka executable not found in PATH; install Sofka and try again: %w", err)
	}
	return sofkaPath, nil
}

func runSofka(sofkaPath string, args []string) error {
	cmd := exec.Command(sofkaPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch sofka: %w", err)
	}
	return nil
}
