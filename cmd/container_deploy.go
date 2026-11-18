package cmd

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var containerDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		validateContainerArguments(containerName, containerNames)
		checkError(validateContainerDeployVersion(containerTag, containerCommitId))
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		// deploy multiple services
		containerList := buildContainerListFromContainerNames(client, envId, containerName, containerNames)
		watch := utils.NewServicesWatch(client, envId, containerIds(containerList), watchFlag)
		err := deployContainers(client, envId, containerList, containerTag, containerCommitId)
		checkError(err)
		utils.Println(fmt.Sprintf("Request to deploy container(s) %s has been queued..", pterm.FgBlue.Sprintf("%s%s", containerName, containerNames)))
		watch.Wait(qovery.STATEENUM_DEPLOYED)
	},
}

func validateContainerDeployVersion(tag string, commitId string) error {
	if tag != "" && commitId != "" {
		return fmt.Errorf("use either --tag or --commit-id, not both")
	}

	if utils.IsLatestCommitKeyword(commitId) {
		return fmt.Errorf("--commit-id %s is not supported for Git containers: pass a commit SHA", utils.LatestCommitKeyword)
	}

	return nil
}

func deployContainers(client *qovery.APIClient, envId string, containerList []*qovery.ContainerResponse, tag string, commitId string) error {
	if commitId != "" {
		return utils.DeployContainersAtCommit(client, containerList, commitId)
	}

	return utils.DeployContainers(client, envId, containerList, tag)
}

func init() {
	containerCmd.AddCommand(containerDeployCmd)
	containerDeployCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerDeployCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerDeployCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerDeployCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerDeployCmd.Flags().StringVarP(&containerNames, "containers", "", "", "Container Names (comma separated) (ex: --containers \"container1,container2\")")
	containerDeployCmd.Flags().StringVarP(&containerTag, "tag", "t", "", "Container Tag (image containers only)")
	containerDeployCmd.Flags().StringVarP(&containerCommitId, "commit-id", "c", "", "Container Commit ID (Git containers only)")
	containerDeployCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch container status until it's ready or an error occurs")
}
