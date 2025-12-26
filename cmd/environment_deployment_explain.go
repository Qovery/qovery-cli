package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"github.com/xlab/treeprint"
	"os"
	"time"
)

var level string

const (
	StageLevel       = 1
	ServiceLevel     = 2
	StepLevel        = 3
	MessageLevel     = 4
	AllLevel     int = 5
)

var environmentDeploymentExplainCmd = &cobra.Command{
	Use:   "explain",
	Short: "Explain environment deployment -- give details about what happened during the deployment",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if level != "" && level != "all" && level != "stage" && level != "service" && level != "step" && level != "message" {
			utils.PrintlnError(fmt.Errorf("invalid value for --show-only: %s", level))
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

		environment, _, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), environmentId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		logsQuery := client.EnvironmentLogsAPI.ListEnvironmentLogs(context.Background(), environmentId)
		if id != "" {
			logsQuery = logsQuery.Version(id)
		}

		logs, _, err := logsQuery.Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		mLevel := AllLevel
		switch level {
		case "", "all":
			mLevel = AllLevel
		case "stage":
			mLevel = StageLevel
		case "service":
			mLevel = ServiceLevel
		case "step":
			mLevel = StepLevel
		case "message":
			mLevel = MessageLevel
		}

		tree := treeprint.New()
		envBranch := tree.AddBranch(fmt.Sprintf("Environment: %s [duration: %s]", environment.Name, getDurationFromLogs(logs)))

		branchByStage := make(map[string]treeprint.Tree)

		for stageIdx, stage := range getStagesFromLogs(logs) {
			stageStartTime, stageEndTime := getStartTimeAndEndTimeByStage(stage, logs)
			branch := envBranch.AddBranch(fmt.Sprintf("Stage %d: %s [duration: %s]", stageIdx+1, stage, utils.GetDuration(stageStartTime, stageEndTime)))
			branchByStage[stage] = branch

			if mLevel >= ServiceLevel {
				for _, service := range getServicesFromLogsByStage(stage, logs) {
					serviceStartTime, serviceEndTime := getStartTimeAndEndTimeByServiceAndStage(service, stage, logs)
					serviceBranch := branch.AddBranch(fmt.Sprintf("%s [duration: %s]", service, utils.GetDuration(serviceStartTime, serviceEndTime)))

					if mLevel >= StepLevel {
						stepIdx := 0
						for _, step := range getStepsFromLogsByService(service, logs) {
							stepStartTime, stepEndTime := getStepStartTimeAndEndTimeFromLogsByServiceAndStep(service, step, logs)
							if stepEndTime.Sub(stepStartTime).Seconds() > 0 {
								stepIdx++
								// only display if step took more than 0 seconds
								stepBranch := serviceBranch.AddBranch(fmt.Sprintf("Step %d: %s [duration: %s]", stepIdx, step, utils.GetDuration(stepStartTime, stepEndTime)))

								if mLevel >= MessageLevel {
									for _, stepLog := range filterLogsByServiceAndStep(service, step, logs) {
										message := stepLog.GetMessage()
										stepBranch.AddNode(message.GetSafeMessage())
									}
								}
							}
						}
					}
				}
			}
		}

		fmt.Println(tree.String())

		//var data [][]string
		//
		//for _, log := range logs {
		//	message := log.GetMessage()
		//	data = append(data, []string{
		//		log.Timestamp.String(),
		//		log.Details.StageLevel.GetName(),
		//		log.Details.StageLevel.GetStep(),
		//		log.Details.Transmitter.GetName(),
		//		log.Details.Transmitter.GetType(),
		//		message.GetSafeMessage(),
		//	})
		//}
		//
		//err = utils.PrintTable([]string{"Timestamp", "StageLevel", "StepLevel", "ServiceLevel", "ServiceLevel Type", "Message"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func getDurationFromLogs(logs []qovery.EnvironmentLogs) string {
	var startTime time.Time
	var endTime time.Time

	for _, log := range logs {
		if startTime.IsZero() || startTime.After(log.Timestamp) {
			startTime = log.Timestamp
		}

		if endTime.IsZero() || endTime.Before(log.Timestamp) {
			endTime = log.Timestamp
		}
	}

	return utils.GetDuration(startTime, endTime)
}

func getStagesFromLogs(logs []qovery.EnvironmentLogs) []string {
	stages := make(map[string]bool)
	var stagesList []string

	for _, log := range logs {
		stageName := log.Details.Stage.GetName()
		if _, ok := stages[stageName]; !ok {
			stages[stageName] = true
			stagesList = append(stagesList, stageName)
		}
	}

	return stagesList
}

func getStartTimeAndEndTimeByStage(stage string, logs []qovery.EnvironmentLogs) (time.Time, time.Time) {
	var startTime time.Time
	var endTime time.Time

	for _, log := range logs {
		if log.Details.Stage.GetName() == stage {
			if startTime.IsZero() || startTime.After(log.Timestamp) {
				startTime = log.Timestamp
			}

			if endTime.IsZero() || endTime.Before(log.Timestamp) {
				endTime = log.Timestamp
			}
		}
	}

	return startTime, endTime
}

func getStartTimeAndEndTimeByServiceAndStage(service string, stage string, logs []qovery.EnvironmentLogs) (time.Time, time.Time) {
	var startTime time.Time
	var endTime time.Time

	for _, log := range logs {
		if log.Details.Stage.GetName() == stage && log.Details.Transmitter.GetName() == service {
			if startTime.IsZero() || startTime.After(log.Timestamp) {
				startTime = log.Timestamp
			}

			if endTime.IsZero() || endTime.Before(log.Timestamp) {
				endTime = log.Timestamp
			}
		}
	}

	return startTime, endTime
}

func getServicesFromLogsByStage(stage string, logs []qovery.EnvironmentLogs) []string {
	services := make(map[string]bool)
	var servicesList []string

	for _, log := range logs {
		serviceName := log.Details.Transmitter.GetName()
		if log.Details.Stage.GetName() == stage && log.Details.Transmitter.GetType() != "Environment" {
			if _, ok := services[serviceName]; !ok {
				services[serviceName] = true
				servicesList = append(servicesList, serviceName)
			}
		}
	}

	return servicesList
}

func getStepsFromLogsByService(service string, logs []qovery.EnvironmentLogs) []string {
	steps := make(map[string]bool)
	var stepsList []string

	for _, log := range logs {
		stepName := log.Details.Stage.GetStep()
		if log.Details.Transmitter.GetName() == service {
			if _, ok := steps[stepName]; !ok {
				steps[stepName] = true
				stepsList = append(stepsList, stepName)
			}
		}
	}

	return stepsList
}

func getStepStartTimeAndEndTimeFromLogsByServiceAndStep(service string, step string, logs []qovery.EnvironmentLogs) (time.Time, time.Time) {
	var startTime time.Time
	var endTime time.Time

	for _, log := range logs {
		if log.Details.Transmitter.GetName() == service && log.Details.Stage.GetStep() == step {
			if startTime.IsZero() || startTime.After(log.Timestamp) {
				startTime = log.Timestamp
			}

			if endTime.IsZero() || endTime.Before(log.Timestamp) {
				endTime = log.Timestamp
			}
		}
	}

	return startTime, endTime
}

func filterLogsByServiceAndStep(service string, step string, logs []qovery.EnvironmentLogs) []qovery.EnvironmentLogs {
	var filteredLogs []qovery.EnvironmentLogs

	for _, log := range logs {
		if log.Details.Transmitter.GetName() == service && log.Details.Stage.GetStep() == step {
			filteredLogs = append(filteredLogs, log)
		}
	}

	return filteredLogs
}

func init() {
	environmentDeploymentCmd.AddCommand(environmentDeploymentExplainCmd)
	environmentDeploymentExplainCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	environmentDeploymentExplainCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	environmentDeploymentExplainCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	environmentDeploymentExplainCmd.Flags().StringVarP(&id, "id", "", "", "Deployment Id")
	environmentDeploymentExplainCmd.Flags().StringVarP(&level, "level", "", "all", "Show only: <all|stage|service|step|message> (default: all)")
}
