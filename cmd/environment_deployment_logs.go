package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	deploymentLogsExecutionId string
	deploymentLogsServiceName string
	deploymentLogsServiceId   string
	deploymentLogsPreCheck    bool
	deploymentLogsErrorsOnly  bool
)

var environmentDeploymentLogsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Print the deployment logs of an environment",
	Long: `Print the deployment logs of an environment -- the engine output shown in the "Deployment logs" tab of the console.

These are the logs of a deployment itself (build, deploy, health checks), not the runtime output of a
running service: for that, use "qovery log".

Without --execution-id, the logs of the latest deployment are printed. The API returns at most the
last 1000 lines.

--service matches the service name carried by each log line, so it works for every service type
(including Terraform services). Use --service-id to match on the service uuid instead; the two are mutually exclusive. Either way,
the environment-level lines are kept for context.

Note: the console additionally scopes lines to the deployment stage the service belongs to, which
the CLI cannot resolve without an extra lookup, so a few stage-level lines from other stages may
appear in a service-scoped view.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if deploymentLogsPreCheck && (deploymentLogsServiceName != "" || deploymentLogsServiceId != "") {
			printlnErrorToStderr(fmt.Errorf("--pre-check cannot be combined with --service or --service-id: pre-check runs before any service is deployed"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// They are two spellings of one selector, and the filter matches either, so passing
		// both would widen the result to two services rather than narrow it.
		if deploymentLogsServiceName != "" && deploymentLogsServiceId != "" {
			printlnErrorToStderr(fmt.Errorf("--service and --service-id select the same thing two different ways: pass one, not both"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			printlnErrorToStderr(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		if err != nil {
			printlnErrorToStderr(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		logsQuery := client.EnvironmentLogsAPI.ListEnvironmentLogs(context.Background(), environmentId)
		if deploymentLogsExecutionId != "" {
			logsQuery = logsQuery.Version(deploymentLogsExecutionId)
		}

		logs, _, err := logsQuery.Execute()
		if err != nil {
			printlnErrorToStderr(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		filter := pkg.DeploymentLogFilter{
			ServiceID:    deploymentLogsServiceId,
			ServiceName:  deploymentLogsServiceName,
			PreCheckOnly: deploymentLogsPreCheck,
			ErrorsOnly:   deploymentLogsErrorsOnly,
		}

		all := logs

		// Apply the scope first and the error filter second, so the two reasons a result can
		// come back empty stay distinguishable: the service matched nothing, or it matched and
		// simply had no errors. Collapsing them reports a healthy service as a missing one.
		scopeFilter := filter
		scopeFilter.ErrorsOnly = false
		scoped := pkg.FilterDeploymentLogs(all, scopeFilter)

		logs = scoped
		if filter.ErrorsOnly {
			logs = pkg.FilterDeploymentLogs(scoped, pkg.DeploymentLogFilter{ErrorsOnly: true})
		}

		// All of these go to stderr, not stdout: the log lines are this command's output and
		// are routinely piped into jq or a file.
		switch {
		case filter.IsServiceScoped() && !pkg.HasServiceSpecificLines(scoped):
			// The service contributed nothing, so only environment-wide lines remain -- an
			// empty-looking success. Name what did emit, so the user can retry.
			if services := pkg.DeploymentLogServices(all); len(services) == 0 {
				printlnInfoToStderr("No service emitted deployment logs in this deployment.")
			} else {
				printlnInfoToStderr(fmt.Sprintf(
					"No deployment logs for that service. Services in this deployment: %s",
					strings.Join(services, ", "),
				))
			}
		case filter.ErrorsOnly && len(logs) == 0 && filter.IsServiceScoped():
			printlnInfoToStderr("No errors for that service in this deployment.")
		case filter.ErrorsOnly && len(logs) == 0:
			printlnInfoToStderr("No errors in this deployment.")
		}

		if jsonFlag {
			projected := make([]map[string]interface{}, 0, len(logs))
			for _, l := range logs {
				projected = append(projected, pkg.DeploymentLogJSON(l))
			}

			out, err := json.Marshal(projected)
			if err != nil {
				printlnErrorToStderr(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}

			fmt.Println(string(out))
			return
		}

		for _, l := range logs {
			fmt.Println(pkg.FormatDeploymentLogLine(l))
		}
	},
}

// printlnInfoToStderr and printlnErrorToStderr mirror utils.PrintlnInfo / utils.PrintlnError
// but write to stderr. The shared helpers print to stdout, which is fine for commands whose
// output is prose but not for this one: stdout carries the log lines, and an "Error: ..."
// or "Info: ..." line mixed into them breaks `--json | jq`. Kept local rather than changing
// the shared helpers, which every other command depends on.
func printlnInfoToStderr(info string) {
	_, _ = fmt.Fprintf(os.Stderr, "%v: %v\n", color.CyanString("Info"), info)
}

func printlnErrorToStderr(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", color.RedString("Error"), err)
}

func init() {
	environmentDeploymentCmd.AddCommand(environmentDeploymentLogsCmd)
	environmentDeploymentLogsCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeploymentLogsCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeploymentLogsCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeploymentLogsCmd.Flags().StringVarP(&deploymentLogsExecutionId, "execution-id", "", "", "Execution Id of a past deployment, as listed by 'qovery environment deployment list' (default: latest deployment)")
	environmentDeploymentLogsCmd.Flags().StringVarP(&deploymentLogsServiceName, "service", "s", "", "Service Name -- only print the log lines related to that service's deployment")
	environmentDeploymentLogsCmd.Flags().StringVarP(&deploymentLogsServiceId, "service-id", "", "", "Service Id -- same as --service, skipping the name lookup")
	environmentDeploymentLogsCmd.Flags().BoolVarP(&deploymentLogsPreCheck, "pre-check", "", false, "Only print the pre-check log lines")
	environmentDeploymentLogsCmd.Flags().BoolVarP(&deploymentLogsErrorsOnly, "errors-only", "", false, "Only print the log lines carrying an error")
	environmentDeploymentLogsCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "Print the logs as JSON")
}
