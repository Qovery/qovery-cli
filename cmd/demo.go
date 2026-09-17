package cmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

// writeDemoTokenFile keeps the authorization header out of script and curl
// arguments, including shell debug traces. The caller must remove the file.
func writeDemoTokenFile(dir string, tokenType utils.AccessTokenType, token utils.AccessToken) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve demo token directory: %w", err)
	}
	file, err := os.CreateTemp(dir, "qovery-token-*")
	if err != nil {
		return "", fmt.Errorf("create demo token file: %w", err)
	}

	_, writeErr := fmt.Fprintln(file, "Authorization: "+utils.GetAuthorizationHeaderValue(tokenType, token))
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("write demo token file: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("close demo token file: %w", closeErr)
	}
	return file.Name(), nil
}

// prepareDemoTokenFile holds a directory lock until cleanup, so stale token
// files can be removed without deleting another running demo's credentials.
func prepareDemoTokenFile(dir string, tokenType utils.AccessTokenType, token utils.AccessToken) (string, func(), error) {
	lock, err := lockDemoTokenDirectory(dir)
	if err != nil {
		return "", nil, err
	}
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "qovery-token-") {
				if err = os.Remove(filepath.Join(dir, entry.Name())); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		_ = lock.Close()
		return "", nil, fmt.Errorf("remove stale demo token files: %w", err)
	}

	path, err := writeDemoTokenFile(dir, tokenType, token)
	if err != nil {
		_ = lock.Close()
		return "", nil, err
	}
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				utils.PrintlnError(fmt.Errorf("cannot remove demo token file: %w", err))
			}
			_ = lock.Close()
		})
	}
	return path, cleanup, nil
}

var (
	demoClusterName        string
	demoDeleteQoveryConfig bool
	demoDebug              bool
	demoChartPath          string
	demoEngineImage        string
)

//go:embed demo_scripts/create_qovery_demo.sh
var demoScriptsCreate []byte

//go:embed demo_scripts/destroy_qovery_demo.sh
var demoScriptsDestroy []byte

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Try Qovery on your local machine",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(demoCmd)
}
