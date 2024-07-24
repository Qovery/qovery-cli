package cmd

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var demoUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Create a k3s kubernetes cluster with Qovery installed on your local machine",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if runtime.GOOS == "windows" {
			utils.PrintlnError(fmt.Errorf("qovery demo is not supported from Windows. Please use WSL (Windows Subsystem for Linux) to use qovery demo"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		orgId, _, err := utils.CurrentOrganization(true)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot get Bearer or Token to access Qovery API. Please use `qovery auth` first: %s", err))
			utils.PrintlnError(err)
			os.Exit(1)
		}

		regex := "^[a-zA-Z][-a-z]+[a-zA-Z]$"
		match, _ := regexp.MatchString(regex, demoClusterName)
		if !match {
			utils.PrintlnError(fmt.Errorf("cluster name must match regex %s: got %s", regex, demoClusterName))
			os.Exit(1)
		}

		scriptDir := filepath.Join(os.TempDir(), "qovery-demo")
		mErr := os.MkdirAll(scriptDir, os.FileMode(0700))
		if mErr != nil {
			utils.PrintlnError(mErr)
			os.Exit(1)
		}

		scriptPath := filepath.Join(scriptDir, "create_demo_cluster.sh")
		debugLogsPath := filepath.Join(scriptDir, "qovery-demo.log")
		err = os.WriteFile(scriptPath, demoScriptsCreate, 0700)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot write file to disk: %s", err))
			os.Exit(1)
		}

		cmdArgs := fmt.Sprintf("set -euo pipefail ; %s %s %s %s %s %t 2>&1 | tee %s", scriptPath, demoClusterName, strings.ToUpper(runtime.GOARCH), string(orgId), string(token), demoDebug, debugLogsPath)
		shCmd := exec.Command("/bin/sh", "-c", cmdArgs)
		shCmd.Stdout = os.Stdout
		shCmd.Stderr = os.Stderr
		if err := shCmd.Run(); err != nil || !shCmd.ProcessState.Success() {
			utils.PrintlnError(fmt.Errorf("error executing the command %s", err))
			uploadErrorLogs(tokenType, token, orgId, demoClusterName, debugLogsPath)
			utils.CaptureError(cmd, shCmd.String(), err.Error())
		}

		utils.CaptureWithEvent(cmd, utils.EndOfExecutionEventName)
	},
}

func uploadErrorLogs(tokenType utils.AccessTokenType, token utils.AccessToken, organization utils.Id, clusterName string, debugLogsPath string) {
	type Payload struct {
		Organization string    `json:"organization"`
		ClusterName  string    `json:"cluster_name"`
		Content      string    `json:"content"`
		Os           string    `json:"os"`
		CpuArch      string    `json:"cpu_arch"`
		CliVersion   string    `json:"cli_version"`
		Timestamp    time.Time `json:"timestamp"`
	}

	content, _ := os.ReadFile(debugLogsPath)
	payload, _ := json.Marshal(Payload{
		Organization: string(organization),
		ClusterName:  clusterName,
		Content:      string(content),
		Os:           runtime.GOOS,
		CpuArch:      runtime.GOARCH,
		CliVersion:   pkg.Version,
		Timestamp:    time.Now(),
	})
	client := utils.GetQoveryClient(tokenType, token)
	url := fmt.Sprintf("%s/admin/demoDebugLog", client.GetConfig().Servers[0].URL)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	query := req.URL.Query()
	query.Add("organization", string(organization))
	query.Add("clusterName", clusterName)
	req.URL.RawQuery = query.Encode()

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		utils.PrintlnError(fmt.Errorf("error uploading debug logs: %s", err))
		return
	}

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		utils.PrintlnError(fmt.Errorf("error uploading debug logs: %s %s", response.Status, body))
		return
	}
}

func init() {
	var userName string
	currentUser, err := user.Current()
	if err != nil {
		userName = "qovery"
	} else {
		userName = currentUser.Username
	}

	var demoUpCmd = demoUpCmd
	demoUpCmd.Flags().StringVarP(&demoClusterName, "cluster-name", "c", "local-demo-"+userName, "The name of the cluster to create")
	demoUpCmd.Flags().BoolVar(&demoDebug, "debug", false, "Enable debug mode")

	demoCmd.AddCommand(demoUpCmd)
}
