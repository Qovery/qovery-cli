package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var applicationCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone an application",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		applications, _, err := client.ApplicationsApi.ListApplication(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		application := utils.FindByApplicationName(applications.GetResults(), applicationName)

		if application == nil {
			utils.PrintlnError(fmt.Errorf("application %s not found", applicationName))
			utils.PrintlnInfo("You can list all applications with: qovery application list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		sourceEnvironment, _, err := client.EnvironmentMainCallsApi.GetEnvironment(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		environments, _, err := client.EnvironmentsApi.ListEnvironment(context.Background(), sourceEnvironment.Project.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		if targetEnvironmentName == "" {
			// use same env name as the source env
			targetEnvironmentName = sourceEnvironment.Name
		}

		targetEnvironment := utils.FindByEnvironmentName(environments.GetResults(), targetEnvironmentName)

		if targetEnvironment == nil {
			utils.PrintlnError(fmt.Errorf("environment %s not found", targetEnvironmentName))
			utils.PrintlnInfo("You can list all environments with: qovery environment list")
			os.Exit(1)
		}

		var storage []qovery.ServiceStorageRequestStorageInner

		for _, s := range application.Storage {
			storage = append(storage, qovery.ServiceStorageRequestStorageInner{
				Type:       s.Type,
				Size:       s.Size,
				MountPoint: s.MountPoint,
			})
		}

		var ports []qovery.ServicePortRequestPortsInner

		for _, p := range application.Ports {
			ports = append(ports, qovery.ServicePortRequestPortsInner{
				Name:               p.Name,
				InternalPort:       p.InternalPort,
				ExternalPort:       p.ExternalPort,
				PubliclyAccessible: p.PubliclyAccessible,
				IsDefault:          p.IsDefault,
				Protocol:           &p.Protocol,
			})
		}

		if targetApplicationName == "" {
			targetApplicationName = *application.Name
		}

		var gitRepository qovery.ApplicationGitRepositoryRequest

		if application.GitRepository != nil {
			gitRepository = qovery.ApplicationGitRepositoryRequest{
				Url:      *application.GitRepository.Url,
				Branch:   application.GitRepository.Branch,
				RootPath: application.GitRepository.RootPath,
			}
		}

		req := qovery.ApplicationRequest{
			Storage:             storage,
			Ports:               ports,
			Name:                targetApplicationName,
			Description:         application.Description,
			GitRepository:       gitRepository,
			BuildMode:           application.BuildMode,
			DockerfilePath:      application.DockerfilePath,
			BuildpackLanguage:   application.BuildpackLanguage,
			Cpu:                 application.Cpu,
			Memory:              application.Memory,
			MinRunningInstances: application.MinRunningInstances,
			MaxRunningInstances: application.MaxRunningInstances,
			Healthcheck:         application.Healthcheck,
			AutoPreview:         application.AutoPreview,
			Arguments:           application.Arguments,
			Entrypoint:          application.Entrypoint,
		}

		createdService, res, err := client.ApplicationsApi.CreateApplication(context.Background(), targetEnvironment.Id).ApplicationRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)

			bodyBytes, err := io.ReadAll(res.Body)
			if err != nil {
				return
			}

			utils.PrintlnError(fmt.Errorf("unable to clone application %s", string(bodyBytes)))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		deploymentStageId := utils.GetDeploymentStageId(client, application.Id)

		_, _, err = client.DeploymentStageMainCallsApi.AttachServiceToDeploymentStage(context.Background(), deploymentStageId, createdService.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// clone advanced settings
		settings, _, err := client.ApplicationConfigurationApi.GetAdvancedSettings(context.Background(), application.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, _, err = client.ApplicationConfigurationApi.EditAdvancedSettings(context.Background(), createdService.Id).ApplicationAdvancedSettings(*settings).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Application %s cloned!", pterm.FgBlue.Sprintf(applicationName)))
	},
}

func init() {
	applicationCmd.AddCommand(applicationCloneCmd)
	applicationCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	applicationCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	applicationCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	applicationCloneCmd.Flags().StringVarP(&applicationName, "application", "n", "", "Application Name")
	applicationCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	applicationCloneCmd.Flags().StringVarP(&targetApplicationName, "target-application-name", "", "", "Target Application Name")

	_ = applicationCloneCmd.MarkFlagRequired("application")
}
