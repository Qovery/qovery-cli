package application

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

func ApplicationUpdate(client *qovery.APIClient, envId string, applicationName string, applicationBranch string, applicationAutoDeploy bool, changeAutoDeploy bool) {
	applications, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()

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

	var storage []qovery.ServiceStorageRequestStorageInner
	for _, s := range application.Storage {
		storage = append(storage, qovery.ServiceStorageRequestStorageInner{
			Id:         &s.Id,
			Type:       s.Type,
			Size:       s.Size,
			MountPoint: s.MountPoint,
		})
	}

	req := qovery.ApplicationEditRequest{
		Storage:     storage,
		Name:        &application.Name,
		Description: application.Description,
		GitRepository: &qovery.ApplicationGitRepositoryRequest{
			Branch:     application.GitRepository.Branch,
			GitTokenId: application.GitRepository.GitTokenId,
			RootPath:   application.GitRepository.RootPath,
			Url:        application.GitRepository.Url,
		},
		BuildMode:           application.BuildMode,
		DockerfilePath:      application.DockerfilePath,
		BuildpackLanguage:   application.BuildpackLanguage,
		Cpu:                 application.Cpu,
		Memory:              application.Memory,
		MinRunningInstances: application.MinRunningInstances,
		MaxRunningInstances: application.MaxRunningInstances,
		Healthchecks:        application.Healthchecks,
		AutoPreview:         application.AutoPreview,
		Ports:               application.Ports,
		Arguments:           application.Arguments,
		Entrypoint:          application.Entrypoint,
		AutoDeploy:          *qovery.NewNullableBool(application.AutoDeploy),
	}

	if applicationBranch != "" {
		req.GitRepository.Branch = &applicationBranch
	}

	if changeAutoDeploy {
		req.AutoDeploy = *qovery.NewNullableBool(&applicationAutoDeploy)
	}

	_, _, err = client.ApplicationMainCallsAPI.EditApplication(context.Background(), application.Id).ApplicationEditRequest(req).Execute()

	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

}
