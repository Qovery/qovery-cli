package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"os"
	"os/exec"

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

	var cleanup func()
	if !doNotConnectToBastion {
		cleanup = pkg.SetBastionConnection()
		defer func() {
			log.Info("Cleaning up SSH tunnel...")
			cleanup()
		}()
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
