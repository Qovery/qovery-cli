package cmd

import (
	_ "embed"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

var demoCmd = &cobra.Command{
	Use:   "demo [up|destroy]",
	Short: "Create a demo kubernetes cluster with Qovery installed on your local machine",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		currentContext, err := utils.CurrentContext()
		if err != nil {
			log.Errorf("Cannot get current Qovery context %s", currentContext)
			os.Exit(1)
		}
		organizationId := string(currentContext.OrganizationId)
		if organizationId == "" {
			log.Errorf("Qovery context is not set. Use `qovery context set` first")
			os.Exit(1)
		}

		_, token, err := utils.GetAccessToken()
		if err != nil {
			log.Errorf("Cannot get Bearer or Token to access Qovery API. Please use `qovery auth` first: %s", err)
			utils.PrintlnError(err)
			os.Exit(1)
		}

		if args[0] == "up" {
			regex := "^[a-zA-Z][-a-z]+[a-zA-Z]$"
			match, _ := regexp.MatchString(regex, demoClusterName)
			if !match {
				log.Errorf("cluster name must match regex %s: got %s", regex, demoClusterName)
				os.Exit(1)
			}

			err := os.WriteFile("create_demo_cluster.sh", demoScriptsCreate, 0700)
			if err != nil {
				log.Errorf("Cannot write file to disk: %s", err)
				os.Exit(1)
			}

			cmd := exec.Command("/bin/sh", "create_demo_cluster.sh", demoClusterName, strings.ToUpper(runtime.GOARCH), organizationId, string(token))
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Errorf("Error executing the command %s", err)
			}
			os.Exit(0)
		}

		if args[0] == "destroy" {
			err := os.WriteFile("destroy_demo_cluster.sh", demoScriptsDestroy, 0700)
			if err != nil {
				log.Errorf("Cannot write file to disk: %s", err)
				os.Exit(1)
			}

			cmd := exec.Command("/bin/sh", "destroy_demo_cluster.sh", demoClusterName, organizationId, string(token), strconv.FormatBool(demoDeleteQoveryConfig))
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
