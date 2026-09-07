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
(including Terraform services). Use --service-id to match on the service uuid instead. Either way,
the environment-level lines are kept for context.

Note: the console additionally scopes lines to the deployment stage the service belongs to, which
the CLI cannot resolve without an extra lookup, so a few stage-level lines from other stages may
appear in a service-scoped view.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if deploymentLogsPreCheck && (deploymentLogsServiceName != "" || deploymentLogsServiceId != "") {
			utils.PrintlnError(fmt.Errorf("--pre-check cannot be combined with --service or --service-id: pre-check runs before any service is deployed"))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		logsQuery := client.EnvironmentLogsAPI.ListEnvironmentLogs(context.Background(), environmentId)
		if deploymentLogsExecutionId != "" {
			logsQuery = logsQuery.Version(deploymentLogsExecutionId)
		}

		logs, _, err := logsQuery.Execute()
		if err != nil {
			utils.PrintlnError(err)
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
		logs = pkg.FilterDeploymentLogs(all, filter)

		// A --service that matches nothing leaves only the environment-wide lines, which
		// looks like a successful but empty result. Say so, and name the services that did
		// emit something -- the service may simply not be part of this deployment.
		if filter.IsServiceScoped() && !pkg.HasServiceSpecificLines(logs) {
			// On stderr, not stdout: the logs themselves are the output of this command and
			// are routinely piped into jq or a file.
			if services := pkg.DeploymentLogServices(all); len(services) == 0 {
				printlnInfoToStderr("No service emitted deployment logs in this deployment.")
			} else {
				printlnInfoToStderr(fmt.Sprintf(
					"No deployment logs for that service. Services in this deployment: %s",
					strings.Join(services, ", "),
				))
			}
		}

		if jsonFlag {
			projected := make([]map[string]interface{}, 0, len(logs))
			for _, l := range logs {
				projected = append(projected, pkg.DeploymentLogJSON(l))
			}

			out, err := json.Marshal(projected)
			if err != nil {
				utils.PrintlnError(err)
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

// printlnInfoToStderr mirrors utils.PrintlnInfo but writes to stderr, keeping stdout to the
// log lines alone so that --json stays pipeable.
func printlnInfoToStderr(info string) {
	_, _ = fmt.Fprintf(os.Stderr, "%v: %v\n", color.CyanString("Info"), info)
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
