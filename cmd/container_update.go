package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"io"
	"os"
)

var containerUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a container",
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

		var storage []qovery.ServiceStorageRequestStorageInner
		for _, s := range container.Storage {
			storage = append(storage, qovery.ServiceStorageRequestStorageInner{
				Id:         &s.Id,
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

		imageName := container.ImageName
		if containerImageName != "" {
			imageName = containerImageName
		}

		tag := container.Tag
		if containerTag != "" {
			tag = containerTag
		}

		req := qovery.ContainerRequest{
			Name:                container.Name,
			Description:         container.Description,
			ImageName:           imageName,
			Tag:                 tag,
			RegistryId:          container.Registry.Id,
			Cpu:                 utils.Int32(container.Cpu),
			Memory:              utils.Int32(container.Memory),
			MinRunningInstances: utils.Int32(container.MinRunningInstances),
			MaxRunningInstances: utils.Int32(container.MaxRunningInstances),
			Healthchecks:        container.Healthchecks,
			AutoPreview:         utils.Bool(container.AutoPreview),
			Ports:               ports,
			Storage:             storage,
		}

		_, res, err := client.ContainerMainCallsApi.EditContainer(context.Background(), container.Id).ContainerRequest(req).Execute()

		if err != nil {
			// print http body error message
			if res.StatusCode != 200 {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			utils.PrintlnError(err)

			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("Container %s updated!", pterm.FgBlue.Sprintf(containerName)))
	},
}

func init() {
	containerCmd.AddCommand(containerUpdateCmd)
	containerUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerUpdateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerUpdateCmd.Flags().StringVarP(&containerImageName, "image-name", "", "", "Container Image Name")
	containerUpdateCmd.Flags().StringVarP(&containerTag, "tag", "", "", "Container Tag")

	_ = containerUpdateCmd.MarkFlagRequired("container")
}
