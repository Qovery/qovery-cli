package cmd

import (
	"context"
	"fmt"
	"github.com/pterm/pterm"
	"io"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var containerCloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		containers, _, err := client.ContainersApi.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		container := utils.FindByContainerName(containers.GetResults(), containerName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
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

		for _, s := range container.Storage {
			storage = append(storage, qovery.ServiceStorageRequestStorageInner{
				Type:       s.Type,
				Size:       s.Size,
				MountPoint: s.MountPoint,
			})
		}

		var ports []qovery.ServicePortRequestPortsInner

		for _, p := range container.Ports {
			ports = append(ports, qovery.ServicePortRequestPortsInner{
				Name:               p.Name,
				InternalPort:       p.InternalPort,
				ExternalPort:       p.ExternalPort,
				PubliclyAccessible: p.PubliclyAccessible,
				IsDefault:          p.IsDefault,
				Protocol:           &p.Protocol,
			})
		}

		if targetContainerName == "" {
			targetContainerName = container.Name
		}

		req := qovery.ContainerRequest{
			Storage:             storage,
			Ports:               ports,
			Name:                targetContainerName,
			Description:         container.Description,
			RegistryId:          container.Registry.Id,
			ImageName:           container.ImageName,
			Tag:                 container.Tag,
			Arguments:           container.Arguments,
			Entrypoint:          container.Entrypoint,
			Cpu:                 &container.Cpu,
			Memory:              &container.Memory,
			MinRunningInstances: &container.MinRunningInstances,
			MaxRunningInstances: &container.MaxRunningInstances,
			AutoPreview:         &container.AutoPreview,
		}

		createdService, res, err := client.ContainersApi.CreateContainer(context.Background(), targetEnvironment.Id).ContainerRequest(req).Execute()

		if err != nil {
			utils.PrintlnError(err)

			bodyBytes, err := io.ReadAll(res.Body)
			if err != nil {
				return
			}

			utils.PrintlnError(fmt.Errorf("unable to clone container %s", string(bodyBytes)))
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		deploymentStageId := utils.GetDeploymentStageId(client, container.Id)

		_, _, err = client.DeploymentStageMainCallsApi.AttachServiceToDeploymentStage(context.Background(), deploymentStageId, createdService.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// clone advanced settings
		settings, _, err := client.ContainerConfigurationApi.GetContainerAdvancedSettings(context.Background(), container.Id).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		_, _, err = client.ContainerConfigurationApi.EditContainerAdvancedSettings(context.Background(), createdService.Id).ContainerAdvancedSettings(*settings).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Container %s cloned!", pterm.FgBlue.Sprintf(containerName)))
	},
}

func init() {
	containerCmd.AddCommand(containerCloneCmd)
	containerCloneCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerCloneCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerCloneCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerCloneCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerCloneCmd.Flags().StringVarP(&targetEnvironmentName, "target-environment", "", "", "Target Environment Name")
	containerCloneCmd.Flags().StringVarP(&targetContainerName, "target-container-name", "", "", "Target Container Name")

	_ = containerCloneCmd.MarkFlagRequired("container")
}
