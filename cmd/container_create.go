package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var containerRegistryId string
var containerPort int32
var containerCpu int32
var containerMemory int32
var containerMinRunningInstances int32
var containerMaxRunningInstances int32

var containerCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a container service",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		utils.CheckError(err)

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)
		utils.CheckError(err)

		var ports []qovery.ServicePortRequestPortsInner
		if containerPort > 0 {
			portName := fmt.Sprintf("p%d", containerPort)
			protocol := qovery.PORTPROTOCOLENUM_HTTP
			ports = append(ports, qovery.ServicePortRequestPortsInner{
				Name:               &portName,
				InternalPort:       containerPort,
				ExternalPort:       utils.Int32(443),
				PubliclyAccessible: true,
				IsDefault:          utils.Bool(true),
				Protocol:           &protocol,
			})
		}

		req := qovery.ContainerRequest{
			Name:                containerName,
			RegistryId:          containerRegistryId,
			ImageName:           containerImageName,
			Tag:                 containerTag,
			Ports:               ports,
			Cpu:                 utils.Int32(containerCpu),
			Memory:              utils.Int32(containerMemory),
			MinRunningInstances: utils.Int32(containerMinRunningInstances),
			MaxRunningInstances: utils.Int32(containerMaxRunningInstances),
			Healthchecks:        *qovery.NewHealthcheck(),
		}

		created, res, err := client.ContainersAPI.CreateContainer(context.Background(), envId).ContainerRequest(req).Execute()
		if err != nil && res != nil && res.StatusCode != 201 {
			result, _ := io.ReadAll(res.Body)
			utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
		}
		utils.CheckError(err)

		var publicLink string
		if len(ports) > 0 {
			links, _, err := client.ContainerMainCallsAPI.ListContainerLinks(context.Background(), created.Id).Execute()
			if err == nil {
				for _, link := range links.GetResults() {
					publicLink = link.Url
					break
				}
			}
		}

		if jsonFlag {
			out := struct {
				Id         string `json:"id"`
				Name       string `json:"name"`
				PublicLink string `json:"public_link,omitempty"`
			}{Id: created.Id, Name: created.Name, PublicLink: publicLink}
			j, err := json.Marshal(out)
			utils.CheckError(err)
			utils.Println(string(j))
			return
		}

		msg := fmt.Sprintf("Container service %s created! (id: %s)", pterm.FgBlue.Sprintf("%s", created.Name), pterm.FgBlue.Sprintf("%s", created.Id))
		if publicLink != "" {
			msg += fmt.Sprintf(" - Public link: %s", pterm.FgBlue.Sprintf("%s", publicLink))
		}
		utils.Println(msg)
	},
}

func init() {
	containerCmd.AddCommand(containerCreateCmd)
	containerCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerCreateCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerCreateCmd.Flags().StringVarP(&containerRegistryId, "registry", "", "", "Container Registry ID")
	containerCreateCmd.Flags().StringVarP(&containerImageName, "image-name", "", "", "Container Image Name")
	containerCreateCmd.Flags().StringVarP(&containerTag, "tag", "t", "", "Container Image Tag")
	containerCreateCmd.Flags().Int32VarP(&containerPort, "port", "p", 0, "Container Port (0 = no port exposed)")
	containerCreateCmd.Flags().Int32VarP(&containerCpu, "cpu", "", 500, "CPU in millicores (e.g. 500 = 0.5 vCPU)")
	containerCreateCmd.Flags().Int32VarP(&containerMemory, "memory", "", 512, "Memory in MB")
	containerCreateCmd.Flags().Int32VarP(&containerMinRunningInstances, "min-instances", "", 1, "Minimum number of running instances")
	containerCreateCmd.Flags().Int32VarP(&containerMaxRunningInstances, "max-instances", "", 1, "Maximum number of running instances")
	containerCreateCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")

	_ = containerCreateCmd.MarkFlagRequired("container")
	_ = containerCreateCmd.MarkFlagRequired("registry")
	_ = containerCreateCmd.MarkFlagRequired("image-name")
	_ = containerCreateCmd.MarkFlagRequired("tag")
}
