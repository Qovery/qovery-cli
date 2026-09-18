package cmd

import (
	_ "embed"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/qovery/qovery-cli/utils"
)

var demoDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Remove k3s cluster with Qovery installed on your local machine",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		orgId, _, err := utils.CurrentOrganization(true)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot get Bearer or Token to access Qovery API. Please use `qovery auth` first: %s", err))
			os.Exit(1)
		}

		scriptDir := filepath.Join(os.TempDir(), "qovery-demo")
		mErr := os.MkdirAll(scriptDir, os.FileMode(0700))
		if mErr != nil {
			utils.PrintlnError(mErr)
			os.Exit(1)
		}

		scriptPath := filepath.Join(scriptDir, "destroy_demo_cluster.sh")
		err = os.WriteFile(scriptPath, demoScriptsCreate, 0700)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot write file to disk: %s", err))
			os.Exit(1)
		}
		err = os.WriteFile(scriptPath, demoScriptsDestroy, 0700)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot write file to disk: %s", err))
			os.Exit(1)
		}

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		tokenPath, cleanupToken, err := prepareDemoTokenFile(scriptDir, tokenType, token)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		defer cleanupToken()

		shCmd := exec.CommandContext(ctx, "/bin/sh", scriptPath, demoClusterName, string(orgId), tokenPath, strconv.FormatBool(demoDeleteQoveryConfig))
		shCmd.Stdout = os.Stdout
		shCmd.Stderr = os.Stderr
		err = shCmd.Run()
		cleanupToken()
		stop()
		if err != nil {
			utils.PrintlnError(fmt.Errorf("error executing the command %s", err))
			utils.CaptureError(cmd, shCmd.String(), err.Error())
		}
		utils.CaptureWithEvent(cmd, utils.EndOfExecutionEventName)
	},
}

func init() {
	var userName string
	currentUser, err := user.Current()
	if err != nil {
		userName = "qovery"
	} else {
		userName = currentUser.Username
	}

	var demoDestroyCmd = demoDestroyCmd
	demoDestroyCmd.Flags().StringVarP(&demoClusterName, "cluster-name", "c", "local-demo-"+userName, "The name of the cluster to destroy")
	demoDestroyCmd.Flags().BoolVarP(&demoDeleteQoveryConfig, "delete-qovery-config", "d", false, "Delete the config on Qovery side as well (environments and associated cluster)")

	demoCmd.AddCommand(demoDestroyCmd)
}
