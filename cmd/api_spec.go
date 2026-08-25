package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

// openAPISpecURL points at the canonical source of truth for the Qovery API spec.
// There is no dedicated docs site serving the raw file (api-doc.qovery.com now
// redirects to the rendered docs), so the GitHub repo itself is the only stable
// place to fetch it from.
const openAPISpecURL = "https://raw.githubusercontent.com/Qovery/qovery-openapi-spec/main/openapi.yaml"

var apiSpecOutput string

var apiSpecCmd = &cobra.Command{
	Use:   "spec",
	Short: "Print the Qovery API's OpenAPI specification",
	Long: `Fetch and print the Qovery API's OpenAPI specification (YAML), sourced from
https://github.com/Qovery/qovery-openapi-spec.

Use it to discover valid endpoints, methods, and request/response shapes before
calling 'qovery api <endpoint>' — instead of guessing or relying on a copy that
may be out of date. This command does not require authentication.

EXAMPLES

  # Print the spec to stdout
  $ qovery api spec

  # Save it to a file
  $ qovery api spec -o openapi.yaml

  # Look up one path with a YAML query tool
  $ qovery api spec | yq '.paths["/organization"]'`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(openAPISpecURL)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("could not reach %s: %w", openAPISpecURL, err))
			os.Exit(1)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			utils.PrintlnError(fmt.Errorf("failed to fetch OpenAPI spec: server returned %s", resp.Status))
			os.Exit(1)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		if apiSpecOutput != "" {
			if err := os.WriteFile(apiSpecOutput, body, 0644); err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}
			// Status message goes to stderr, not stdout, so stdout stays reserved
			// for the spec itself (the whole point of -o is a clean stdout to script against).
			fmt.Fprintln(os.Stderr, "OpenAPI spec written to "+apiSpecOutput)
			return
		}

		if n, err := os.Stdout.Write(body); err != nil || n != len(body) {
			if err == nil {
				err = fmt.Errorf("short write: wrote %d of %d bytes", n, len(body))
			}
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	apiCmd.AddCommand(apiSpecCmd)
	apiSpecCmd.Flags().StringVarP(&apiSpecOutput, "output", "o", "", "Write the spec to a file instead of stdout")
}
