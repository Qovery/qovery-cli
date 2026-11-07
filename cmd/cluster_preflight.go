package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/qovery/qovery-cli/pkg/cluster/preflight"
	"github.com/qovery/qovery-cli/utils"
)

var clusterPreflightKubeconfig string
var clusterPreflightContext string
var clusterPreflightLokiObjectStorage bool
var clusterPreflightJSON bool
var clusterPreflightInstallPath string

var clusterPreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Check that your Kubernetes cluster is ready before you install Qovery on it",
	Long: `Check that your Kubernetes cluster is ready before you install Qovery on it.

Qovery runs on any Kubernetes cluster, with one of two installations:
  - EKS (AWS), GKE (GCP) and Scaleway Kapsule: the Qovery Operator installation, the Helm
    command shown in the Qovery Console.
  - Every other cluster (AKS, k3s, RKE2, OpenShift, on-premise, ...): the installation with
    the Qovery CLI, qovery cluster install.
    https://www.qovery.com/docs/configuration/integrations/kubernetes/byok-legacy

The preflight detects the provider of your nodes, tells you which installation to use and
runs the checks of that installation. Use --install-path to force one. The checks for an
existing cert-manager, ExternalDNS, Envoy Gateway or Gateway API CRDs fail on the Operator
installation, which does not share them. Both installations install these components, so on
the CLI installation they warn: turn off the Qovery copy in the Helm values file that
qovery cluster install generates, or remove the existing one. An existing qovery Helm release
fails only on the Operator installation; on the CLI installation it is Qovery itself.

The preflight runs against the current context of your kubeconfig, or the one given with
--kubeconfig and --context. It only reads from the cluster: it creates, changes or deletes
nothing, and it reads no Secret data. It needs no Qovery account.

It exits with status 1 when at least one check fails. A failed check would make the
installation or the first deployment of the cluster fail; fix it before installing.
Warnings do not block the installation but can break a feature.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		installPath, err := preflight.ParseInstallPath(clusterPreflightInstallPath)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if clusterPreflightKubeconfig != "" {
			loadingRules.ExplicitPath = clusterPreflightKubeconfig
		}
		overrides := &clientcmd.ConfigOverrides{CurrentContext: clusterPreflightContext}
		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

		rawConfig, err := clientConfig.RawConfig()
		if err != nil {
			utils.PrintlnErrorToStderr(fmt.Errorf("read kubeconfig: %w", err))
			os.Exit(1)
		}
		contextName := rawConfig.CurrentContext
		if clusterPreflightContext != "" {
			contextName = clusterPreflightContext
		}

		restConfig, err := clientConfig.ClientConfig()
		if err != nil {
			utils.PrintlnErrorToStderr(fmt.Errorf("load kubeconfig context %q: %w", contextName, err))
			os.Exit(1)
		}
		restConfig.Timeout = 20 * time.Second

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if !clusterPreflightJSON {
			utils.PrintlnInfo(fmt.Sprintf("Checking context %s (%s). Nothing is changed on the cluster.", contextName, restConfig.Host))
		}

		facts, err := preflight.Collect(ctx, restConfig)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}
		opts := preflight.Options{LokiObjectStorage: clusterPreflightLokiObjectStorage, InstallPath: installPath}
		results := preflight.Evaluate(facts, opts)
		resolvedPath := preflight.ResolveInstallPath(facts, installPath)
		failed := preflight.Failed(results)

		if clusterPreflightJSON {
			output, err := json.MarshalIndent(map[string]any{
				"context":      contextName,
				"server":       restConfig.Host,
				"install_path": resolvedPath,
				"passed":       !failed,
				"checks":       results,
			}, "", "  ")
			if err != nil {
				utils.PrintlnErrorToStderr(err)
				os.Exit(1)
			}
			utils.Println(string(output))
		} else {
			printPreflightResults(results, failed, resolvedPath)
		}

		if failed {
			os.Exit(1)
		}
	},
}

func preflightStatusLabel(status preflight.Status) string {
	switch status {
	case preflight.StatusPass:
		return color.GreenString(string(status))
	case preflight.StatusWarn:
		return color.YellowString(string(status))
	case preflight.StatusFail:
		return color.RedString(string(status))
	default:
		return string(status)
	}
}

func printPreflightResults(results []preflight.Result, failed bool, path preflight.InstallPath) {
	data := make([][]string, 0, len(results))
	for _, r := range results {
		data = append(data, []string{preflightStatusLabel(r.Status), r.Title, r.Detail})
	}
	if err := utils.PrintTable([]string{"Status", "Check", "Detail"}, data); err != nil {
		utils.PrintlnErrorToStderr(err)
		os.Exit(1)
	}

	var toFix []preflight.Result
	for _, r := range results {
		if r.Fix != "" && (r.Status == preflight.StatusFail || r.Status == preflight.StatusWarn) {
			toFix = append(toFix, r)
		}
	}
	if len(toFix) > 0 {
		utils.Println("\nHow to fix:")
		for _, r := range toFix {
			utils.Println(fmt.Sprintf("- [%s] %s (%s): %s", preflightStatusLabel(r.Status), r.Title, r.ID, r.Fix))
		}
	}

	utils.Println("")
	if failed {
		utils.PrintlnError(fmt.Errorf("the cluster is not ready for Qovery. Fix the failed checks, then run qovery cluster preflight again"))
		return
	}
	if path == preflight.InstallPathCLI {
		utils.PrintlnInfo("No blocking issue found. Install Qovery with: qovery cluster install")
		return
	}
	utils.PrintlnInfo("No blocking issue found. You can run the Helm command of the Qovery Operator shown in the Qovery Console.")
}

func init() {
	clusterPreflightCmd.Flags().StringVar(&clusterPreflightKubeconfig, "kubeconfig", "", "Path to the kubeconfig file (default: $KUBECONFIG or ~/.kube/config)")
	clusterPreflightCmd.Flags().StringVar(&clusterPreflightContext, "context", "", "Kubeconfig context to check (default: the current context)")
	clusterPreflightCmd.Flags().BoolVar(&clusterPreflightLokiObjectStorage, "loki-object-storage", false, "Loki will store logs in object storage (s3, gcs or s3Compatible), so no default StorageClass is needed")
	clusterPreflightCmd.Flags().BoolVar(&clusterPreflightJSON, "json", false, "JSON output")
	clusterPreflightCmd.Flags().StringVar(&clusterPreflightInstallPath, "install-path", "auto", "Installation to check for: operator (EKS, GKE, Scaleway Kapsule), cli (qovery cluster install, any other cluster) or auto (picked from the provider of the nodes)")
	clusterCmd.AddCommand(clusterPreflightCmd)
}
