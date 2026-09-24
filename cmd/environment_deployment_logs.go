package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
			utils.PrintlnErrorToStderr(fmt.Errorf("--pre-check cannot be combined with --service or --service-id: pre-check runs before any service is deployed"))
			os.Exit(1)
		}

		// They are two spellings of one selector, and the filter matches either, so passing
		// both would widen the result to two services rather than narrow it.
		if deploymentLogsServiceName != "" && deploymentLogsServiceId != "" {
			utils.PrintlnErrorToStderr(fmt.Errorf("--service and --service-id select the same thing two different ways: pass one, not both"))
			os.Exit(1)
		}

		tokenType, token, err := utils.GetAccessToken(false)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, environmentId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
		}

		logsQuery := client.EnvironmentLogsAPI.ListEnvironmentLogs(context.Background(), environmentId)
		if deploymentLogsExecutionId != "" {
			logsQuery = logsQuery.Version(deploymentLogsExecutionId)
		}

		logs, _, err := logsQuery.Execute()
		if err != nil {
			utils.PrintlnErrorToStderr(err)
			os.Exit(1)
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
				utils.PrintlnInfoToStderr("No service emitted deployment logs in this deployment.")
			} else {
				utils.PrintlnInfoToStderr(fmt.Sprintf(
					"No deployment logs for that service. Services in this deployment: %s",
					strings.Join(services, ", "),
				))
			}
		case filter.ErrorsOnly && len(logs) == 0 && filter.IsServiceScoped():
			utils.PrintlnInfoToStderr("No errors for that service in this deployment.")
		case filter.ErrorsOnly && len(logs) == 0:
			utils.PrintlnInfoToStderr("No errors in this deployment.")
		}

		if jsonFlag {
			projected := make([]map[string]interface{}, 0, len(logs))
			for _, l := range logs {
				projected = append(projected, pkg.DeploymentLogJSON(l))
			}

			out, err := json.Marshal(projected)
			if err != nil {
				utils.PrintlnErrorToStderr(err)
				os.Exit(1)
			}

			fmt.Println(string(out))
			return
		}

		for _, l := range logs {
			fmt.Println(pkg.FormatDeploymentLogLine(l))
		}
	},
}

func init() {
	environmentDeploymentCmd.AddCommand(environmentDeploymentLogsCmd)
	environmentDeploymentLogsCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeploymentLogsCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeploymentLogsCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeploymentLogsCmd.Flags().StringVarP(&deploymentLogsExecutionId, "execution-id", "", "", "Execution Id of a past deployment, as listed by 'qovery environment deployment list' (default: latest deployment). Also accepted as --id, matching 'deployment explain'")

	// `deployment list` prints this id, `deployment explain` takes it as --id, and all three
	// end up in the same Version() call -- so accept that spelling here too rather than making
	// one id need a different flag name per command. A normalizer rather than a second
	// registration, so there is still exactly one flag, one variable and one help entry.
	environmentDeploymentLogsCmd.Flags().SetNormalizeFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if name == "id" {
			name = "execution-id"
		}
		return pflag.NormalizedName(name)
	})
	environmentDeploymentLogsCmd.Flags().StringVarP(&deploymentLogsServiceName, "service", "s", "", "Service Name -- only print the log lines related to that service's deployment")
	environmentDeploymentLogsCmd.Flags().StringVarP(&deploymentLogsServiceId, "service-id", "", "", "Service Id -- same as --service, skipping the name lookup")
	environmentDeploymentLogsCmd.Flags().BoolVarP(&deploymentLogsPreCheck, "pre-check", "", false, "Only print the pre-check log lines")
	environmentDeploymentLogsCmd.Flags().BoolVarP(&deploymentLogsErrorsOnly, "errors-only", "", false, "Only print the log lines carrying an error")
	environmentDeploymentLogsCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "Print the logs as JSON")
}
