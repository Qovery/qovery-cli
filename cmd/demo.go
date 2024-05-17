package cmd

import (
	_ "embed"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var demoCmd = &cobra.Command{
	Use:   "demo [up|destroy]",
	Short: "Create a demo kubernetes cluster with Qovery installed on your local machine",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		orgId, _, err := utils.CurrentOrganization(true)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		tmpDir, err := os.MkdirTemp("", demoClusterName)
		if err != nil {
			log.Errorln("Cannot create temporary directory")
			panic(err)
		}

		if args[0] == "up" {
			regex := "^[a-zA-Z][-a-z]+[a-zA-Z]$"
			match, _ := regexp.MatchString(regex, demoClusterName)
			if !match {
				log.Errorf("cluster name must match regex %s: got %s", regex, demoClusterName)
				os.Exit(1)
			}

			script := tmpDir + "create_demo_cluster.sh"

			err := os.WriteFile(script, demoScriptsCreate, 0700)
			if err != nil {
				log.Errorf("Cannot write file to disk: %s", err)
				os.Exit(1)
			}

			cmd := exec.Command("/bin/sh", script, demoClusterName, strings.ToUpper(runtime.GOARCH), string(orgId), string(token))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Errorf("Error executing the command %s", err)
			}
			os.Exit(0)
		}

		if args[0] == "destroy" {
			script := tmpDir + "destroy_demo_cluster.sh"

			err := os.WriteFile(script, demoScriptsDestroy, 0700)
			if err != nil {
				log.Errorf("Cannot write file to disk: %s", err)
				os.Exit(1)
			}

			cmd := exec.Command("/bin/sh", script, demoClusterName, string(orgId), string(token), strconv.FormatBool(demoDeleteQoveryConfig))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Errorf("Error executing the command %s", err)
			}
			os.Exit(0)
		}

		log.Errorf("Unknown command %s. Only `up` and `destroy` are supported", args[0])
		os.Exit(1)
	},
}
var (
	demoClusterName        string
	demoDeleteQoveryConfig bool
)

//go:embed demo_scripts/create_qovery_demo.sh
var demoScriptsCreate []byte

//go:embed demo_scripts/destroy_qovery_demo.sh
var demoScriptsDestroy []byte

func init() {
	var userName string
	currentUser, err := user.Current()
	if err != nil {
		userName = "qovery"
	} else {
		userName = currentUser.Username
	}

	var demoCmd = demoCmd
	demoCmd.Flags().StringVarP(&demoClusterName, "cluster-name", "c", "local-demo-"+userName, "The name of the cluster to create")
	demoCmd.Flags().BoolVarP(&demoDeleteQoveryConfig, "delete-qovery-config", "d", false, "If you want to delete also the config on Qovery side (environments and associated cluster)")

	rootCmd.AddCommand(demoCmd)
}
