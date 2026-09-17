package cmd

import (
	_ "embed"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

// writeDemoTokenFile keeps the authorization header out of script and curl
// arguments, including shell debug traces. The caller must remove the file.
func writeDemoTokenFile(dir string, tokenType utils.AccessTokenType, token utils.AccessToken) (string, error) {
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
